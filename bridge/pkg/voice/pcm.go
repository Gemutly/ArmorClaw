package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/armorclaw/bridge/pkg/interfaces"
)

type AgentTextBridge interface {
	SendText(ctx context.Context, text string) (string, error)
}

type PCMRouterConfig struct {
	SampleRate       uint32
	VADConfig        EnergyVADConfig
	VADEnabled       bool
}

func DefaultPCMRouterConfig() PCMRouterConfig {
	return PCMRouterConfig{
		SampleRate: 16000,
		VADConfig:  DefaultEnergyVADConfig(),
		VADEnabled: true,
	}
}

type PCMRouter struct {
	config      PCMRouterConfig
	vad         *EnergyThresholdVAD
	stt         Transcriber
	tts         Synthesizer
	agent       AgentTextBridge
	speechBuf   *bytes.Buffer
	state       routerState
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc

	OnSpeechStart   func(timestamp time.Time)
	OnSpeechEnd     func(timestamp time.Time, transcript string)
	OnAgentResponse func(text string)
	OnOutputPCM     func(pcmData []byte)
OnError          func(err error)
}

type routerState int

const (
	routerIdle routerState = iota
	routerListening
	routerProcessing
)

func NewPCMRouter(
	config PCMRouterConfig,
	stt Transcriber,
	tts Synthesizer,
	agent AgentTextBridge,
) *PCMRouter {
	ctx, cancel := context.WithCancel(context.Background())
	return &PCMRouter{
		config:    config,
		vad:       NewEnergyThresholdVAD(config.VADConfig),
		stt:       stt,
		tts:       tts,
		agent:     agent,
		speechBuf: &bytes.Buffer{},
		state:     routerIdle,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (r *PCMRouter) Close() {
	r.cancel()
}

func (r *PCMRouter) ProcessInputPCM(pcmData []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.config.VADEnabled {
		return r.processWithoutVAD(pcmData)
	}

	events := r.vad.ProcessPCM(pcmData)

	for _, evt := range events {
		switch evt.Type {
		case VADEventSpeechStart:
			r.state = routerListening
			r.speechBuf.Reset()
			if r.OnSpeechStart != nil {
				r.OnSpeechStart(evt.Timestamp)
			}

		case VADEventSpeechEnd:
			r.state = routerProcessing
			speechData := r.speechBuf.Bytes()
			r.speechBuf.Reset()
			if r.OnSpeechEnd != nil {
				r.OnSpeechEnd(evt.Timestamp, "")
			}
			go r.processSpeechSegment(speechData)

		case VADEventSilence:
			if r.state == routerListening {
				r.speechBuf.Write(pcmData)
			}
		}
	}

	if r.state == routerListening && len(events) == 0 {
		r.speechBuf.Write(pcmData)
	}

	return nil
}

func (r *PCMRouter) processWithoutVAD(pcmData []byte) error {
	go r.processSpeechSegment(pcmData)
	return nil
}

func (r *PCMRouter) processSpeechSegment(speechData []byte) {
	if len(speechData) == 0 {
		return
	}

	if r.stt == nil {
		if r.OnError != nil {
			r.OnError(fmt.Errorf("STT service not configured"))
		}
		return
	}

	transcript, err := r.stt.Transcribe(r.ctx, speechData)
	if err != nil {
		if r.OnError != nil {
			r.OnError(fmt.Errorf("STT transcription failed: %w", err))
		}
		return
	}

	if transcript.Text == "" {
		r.mu.Lock()
		r.state = routerIdle
		r.mu.Unlock()
		return
	}

	if r.agent == nil {
		if r.OnError != nil {
			r.OnError(fmt.Errorf("agent bridge not configured"))
		}
		return
	}

	responseText, err := r.agent.SendText(r.ctx, transcript.Text)
	if err != nil {
		if r.OnError != nil {
			r.OnError(fmt.Errorf("agent communication failed: %w", err))
		}
		return
	}

	if r.OnAgentResponse != nil {
		r.OnAgentResponse(responseText)
	}

	if r.tts != nil && responseText != "" {
		synthesis, err := r.tts.Synthesize(r.ctx, responseText)
		if err != nil {
			if r.OnError != nil {
				r.OnError(fmt.Errorf("TTS synthesis failed: %w", err))
			}
			return
		}

		if r.OnOutputPCM != nil && synthesis != nil && len(synthesis.AudioData) > 0 {
			r.OnOutputPCM(synthesis.AudioData)
		}
	}

	r.mu.Lock()
	r.state = routerIdle
	r.mu.Unlock()
}

func (r *PCMRouter) State() routerState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

func (r *PCMRouter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vad.Reset()
	r.speechBuf.Reset()
	r.state = routerIdle
}

type StreamReader struct {
	router  *PCMRouter
	buf     *bytes.Buffer
	mu      sync.Mutex
	notify  chan struct{}
	closed  bool
}

func NewStreamReader(router *PCMRouter) *StreamReader {
	sr := &StreamReader{
		router: router,
		buf:    &bytes.Buffer{},
		notify: make(chan struct{}, 1),
	}

	originalCallback := router.OnOutputPCM
	router.OnOutputPCM = func(pcmData []byte) {
		sr.mu.Lock()
		sr.buf.Write(pcmData)
		sr.mu.Unlock()

		select {
		case sr.notify <- struct{}{}:
		default:
		}

		if originalCallback != nil {
			originalCallback(pcmData)
		}
	}

	return sr
}

func (sr *StreamReader) Read(p []byte) (int, error) {
	sr.mu.Lock()
	n, _ := sr.buf.Read(p)
	sr.mu.Unlock()

	if n > 0 {
		return n, nil
	}

	if sr.closed {
		return 0, io.EOF
	}

	select {
	case <-sr.notify:
		sr.mu.Lock()
		n, _ = sr.buf.Read(p)
		sr.mu.Unlock()
		if n > 0 {
			return n, nil
		}
	case <-sr.router.ctx.Done():
		sr.closed = true
		return 0, io.EOF
	case <-time.After(5 * time.Second):
		return 0, nil
	}

	return 0, nil
}

func (sr *StreamReader) Close() error {
	sr.closed = true
	return nil
}

type StreamWriter struct {
	router *PCMRouter
}

func NewStreamWriter(router *PCMRouter) *StreamWriter {
	return &StreamWriter{router: router}
}

func (sw *StreamWriter) Write(p []byte) (int, error) {
	if err := sw.router.ProcessInputPCM(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (sw *StreamWriter) Close() error {
	sw.router.Close()
	return nil
}

type MockAgentTextBridge struct {
	SendTextFunc func(ctx context.Context, text string) (string, error)
}

func (m *MockAgentTextBridge) SendText(ctx context.Context, text string) (string, error) {
	if m.SendTextFunc != nil {
		return m.SendTextFunc(ctx, text)
	}
	return "echo: " + text, nil
}

type MockTranscriber struct {
	TranscribeFunc func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error)
}

func (m *MockTranscriber) Transcribe(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
	if m.TranscribeFunc != nil {
		return m.TranscribeFunc(ctx, audioData)
	}
	return &interfaces.TranscriptionResult{
		Text:       "mock transcription",
		Confidence: 1.0,
		Timestamp:  time.Now(),
	}, nil
}

type MockSynthesizer struct {
	SynthesizeFunc func(ctx context.Context, text string) (*interfaces.SynthesisResult, error)
}

func (m *MockSynthesizer) Synthesize(ctx context.Context, text string) (*interfaces.SynthesisResult, error) {
	if m.SynthesizeFunc != nil {
		return m.SynthesizeFunc(ctx, text)
	}
	return &interfaces.SynthesisResult{
		AudioData:  []byte("mock audio"),
		TextLength: len(text),
		Timestamp:  time.Now(),
	}, nil
}
