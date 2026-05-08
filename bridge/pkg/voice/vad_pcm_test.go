package voice

import (
	"context"
	"testing"
	"time"

	"github.com/armorclaw/bridge/pkg/interfaces"
)

func TestComputeRMS_Silence(t *testing.T) {
	samples := make([]int16, 320)
	rms := ComputeRMS(samples)
	if rms != 0 {
		t.Errorf("expected RMS 0 for silence, got %f", rms)
	}
}

func TestComputeRMS_Tone(t *testing.T) {
	samples := GenerateTone(16000, 440, 0.5, 20)
	rms := ComputeRMS(samples)
	if rms <= 0 {
		t.Fatal("expected non-zero RMS for tone")
	}
	if rms > 0.5 {
		t.Errorf("RMS %f exceeds amplitude 0.5", rms)
	}
}

func TestComputeRMS_Empty(t *testing.T) {
	rms := ComputeRMS(nil)
	if rms != 0 {
		t.Errorf("expected 0 for nil samples, got %f", rms)
	}
}

func TestEnergyThresholdVAD_SpeechDetection(t *testing.T) {
	config := EnergyVADConfig{
		EnergyThreshold:   0.01,
		FrameDurationMs:   20,
		SilenceDurationMs: 300,
		SampleRate:        16000,
	}
	vad := NewEnergyThresholdVAD(config)

	silence := GenerateSilence(16000, 100)
	events := vad.ProcessSamples(silence)
	for _, evt := range events {
		if evt.Type == VADEventSpeechStart {
			t.Error("should not detect speech in silence")
		}
	}
	if vad.State() != VADStateSilence {
		t.Error("should be in silence state after silence input")
	}

	tone := GenerateTone(16000, 440, 0.5, 100)
	events = vad.ProcessSamples(tone)

	foundStart := false
	for _, evt := range events {
		if evt.Type == VADEventSpeechStart {
			foundStart = true
		}
	}
	if !foundStart {
		t.Error("should detect speech start in tone input")
	}
	if vad.State() != VADStateSpeech {
		t.Error("should be in speech state after tone input")
	}
}

func TestEnergyThresholdVAD_SpeechEnd(t *testing.T) {
	config := EnergyVADConfig{
		EnergyThreshold:   0.01,
		FrameDurationMs:   20,
		SilenceDurationMs: 60,
		SampleRate:        16000,
	}
	vad := NewEnergyThresholdVAD(config)

	tone := GenerateTone(16000, 440, 0.5, 100)
	vad.ProcessSamples(tone)

	if vad.State() != VADStateSpeech {
		t.Fatal("should be in speech state after tone")
	}

	silence := GenerateSilence(16000, 200)
	events := vad.ProcessSamples(silence)

	foundEnd := false
	for _, evt := range events {
		if evt.Type == VADEventSpeechEnd {
			foundEnd = true
		}
	}
	if !foundEnd {
		t.Error("should detect speech end after sustained silence")
	}
	if vad.State() != VADStateSilence {
		t.Error("should return to silence state after speech end")
	}
}

func TestEnergyThresholdVAD_Reset(t *testing.T) {
	config := DefaultEnergyVADConfig()
	vad := NewEnergyThresholdVAD(config)

	tone := GenerateTone(16000, 440, 0.5, 100)
	vad.ProcessSamples(tone)
	if vad.State() != VADStateSpeech {
		t.Fatal("should be in speech state")
	}

	vad.Reset()
	if vad.State() != VADStateSilence {
		t.Error("should return to silence after reset")
	}
}

func TestEnergyThresholdVAD_FrameAccumulation(t *testing.T) {
	config := EnergyVADConfig{
		EnergyThreshold:   0.01,
		FrameDurationMs:   20,
		SilenceDurationMs: 300,
		SampleRate:        16000,
	}
	vad := NewEnergyThresholdVAD(config)

	frameSize := config.FrameSize()
	halfFrame := make([]int16, frameSize/2)

	events := vad.ProcessSamples(halfFrame)
	if len(events) != 0 {
		t.Error("should not produce events for incomplete frame")
	}

	tone := GenerateTone(16000, 440, 0.5, 20)
	allSamples := append(halfFrame, tone...)
	events = vad.ProcessSamples(allSamples)

	if len(events) == 0 {
		t.Error("should produce events after accumulated samples complete a frame")
	}
}

func TestEnergyThresholdVAD_DetectSpeech(t *testing.T) {
	config := DefaultEnergyVADConfig()
	vad := NewEnergyThresholdVAD(config)

	silence := GenerateSilence(16000, 100)
	silencePCM := Int16SamplesToBytes(silence)

	result, err := vad.DetectSpeech(context.Background(), silencePCM)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SpeechDetected {
		t.Error("should not detect speech in silence")
	}

	tone := GenerateTone(16000, 440, 0.5, 200)
	tonePCM := Int16SamplesToBytes(tone)

	result, err = vad.DetectSpeech(context.Background(), tonePCM)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.SpeechDetected {
		t.Error("should detect speech in tone")
	}
}

func TestEnergyThresholdVAD_DetectSpeech_EmptyInput(t *testing.T) {
	config := DefaultEnergyVADConfig()
	vad := NewEnergyThresholdVAD(config)

	_, err := vad.DetectSpeech(context.Background(), []byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if err != interfaces.ErrEmptyAudioData {
		t.Errorf("expected ErrEmptyAudioData, got %v", err)
	}
}

func TestBytesToInt16Samples(t *testing.T) {
	original := []int16{100, -100, 0, 32767, -32768}
	pcm := Int16SamplesToBytes(original)
	decoded := BytesToInt16Samples(pcm)

	if len(decoded) != len(original) {
		t.Fatalf("length mismatch: expected %d, got %d", len(original), len(decoded))
	}

	for i, v := range original {
		if decoded[i] != v {
			t.Errorf("sample %d: expected %d, got %d", i, v, decoded[i])
		}
	}
}

func TestGenerateTone(t *testing.T) {
	samples := GenerateTone(16000, 440, 0.5, 100)
	expectedLen := 16000 * 100 / 1000
	if len(samples) != expectedLen {
		t.Errorf("expected %d samples, got %d", expectedLen, len(samples))
	}

	allZero := true
	for _, s := range samples {
		if s != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("tone should contain non-zero samples")
	}
}

func TestGenerateSilence(t *testing.T) {
	samples := GenerateSilence(16000, 100)
	expectedLen := 16000 * 100 / 1000
	if len(samples) != expectedLen {
		t.Errorf("expected %d samples, got %d", expectedLen, len(samples))
	}

	for _, s := range samples {
		if s != 0 {
			t.Error("silence should be all zeros")
		}
	}
}

func TestEnergyVADConfig_FrameSize(t *testing.T) {
	config := EnergyVADConfig{
		FrameDurationMs: 20,
		SampleRate:      16000,
	}
	if config.FrameSize() != 320 {
		t.Errorf("expected frame size 320, got %d", config.FrameSize())
	}
}

func TestEnergyVADConfig_SilenceFrames(t *testing.T) {
	config := EnergyVADConfig{
		FrameDurationMs:   20,
		SilenceDurationMs: 300,
	}
	if config.SilenceFrames() != 15 {
		t.Errorf("expected 15 silence frames, got %d", config.SilenceFrames())
	}
}

func TestPCMRouter_FullPipeline(t *testing.T) {
	config := PCMRouterConfig{
		SampleRate: 16000,
		VADConfig: EnergyVADConfig{
			EnergyThreshold:   0.01,
			FrameDurationMs:   20,
			SilenceDurationMs: 60,
			SampleRate:        16000,
		},
		VADEnabled: true,
	}

	transcriptResult := &interfaces.TranscriptionResult{
		Text:       "hello world",
		Confidence: 0.95,
		Duration:   2 * time.Second,
		Timestamp:  time.Now(),
	}
	synthesisResult := &interfaces.SynthesisResult{
		AudioData:  Int16SamplesToBytes(GenerateTone(16000, 220, 0.3, 500)),
		TextLength: 11,
		Timestamp:  time.Now(),
	}

	mockSTT := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return transcriptResult, nil
		},
	}
	mockTTS := &MockSynthesizer{
		SynthesizeFunc: func(ctx context.Context, text string) (*interfaces.SynthesisResult, error) {
			return synthesisResult, nil
		},
	}
	mockAgent := &MockAgentTextBridge{
		SendTextFunc: func(ctx context.Context, text string) (string, error) {
			return "response: " + text, nil
		},
	}

	router := NewPCMRouter(config, mockSTT, mockTTS, mockAgent)
	defer router.Close()

	var receivedResponse string
	var receivedPCM []byte
	router.OnAgentResponse = func(text string) {
		receivedResponse = text
	}
	router.OnOutputPCM = func(pcmData []byte) {
		receivedPCM = pcmData
	}

	tone := GenerateTone(16000, 440, 0.5, 200)
	tonePCM := Int16SamplesToBytes(tone)
	if err := router.ProcessInputPCM(tonePCM); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	silence := GenerateSilence(16000, 200)
	silencePCM := Int16SamplesToBytes(silence)
	if err := router.ProcessInputPCM(silencePCM); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if receivedResponse != "response: hello world" {
		t.Errorf("expected 'response: hello world', got %q", receivedResponse)
	}
	if len(receivedPCM) == 0 {
		t.Error("expected output PCM data")
	}
}

func TestPCMRouter_VADDisabled(t *testing.T) {
	config := PCMRouterConfig{
		SampleRate:  16000,
		VADEnabled:  false,
		VADConfig:   DefaultEnergyVADConfig(),
	}

	mockSTT := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return &interfaces.TranscriptionResult{
				Text:      "test",
				Timestamp: time.Now(),
			}, nil
		},
	}
	mockAgent := &MockAgentTextBridge{}

	router := NewPCMRouter(config, mockSTT, nil, mockAgent)
	defer router.Close()

	if err := router.ProcessInputPCM([]byte{1, 2, 3, 4}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPCMRouter_STTNil(t *testing.T) {
	config := PCMRouterConfig{
		SampleRate: 16000,
		VADConfig: EnergyVADConfig{
			EnergyThreshold:   0.01,
			FrameDurationMs:   20,
			SilenceDurationMs: 60,
			SampleRate:        16000,
		},
		VADEnabled: true,
	}
	router := NewPCMRouter(config, nil, nil, nil)
	defer router.Close()

	var receivedErr error
	router.OnError = func(err error) {
		receivedErr = err
	}

	tone := GenerateTone(16000, 440, 0.5, 200)
	tonePCM := Int16SamplesToBytes(tone)
	_ = router.ProcessInputPCM(tonePCM)

	silence := GenerateSilence(16000, 200)
	silencePCM := Int16SamplesToBytes(silence)
	_ = router.ProcessInputPCM(silencePCM)

	time.Sleep(200 * time.Millisecond)

	if receivedErr == nil {
		t.Error("expected error for nil STT")
	}
}

func TestPCMRouter_Reset(t *testing.T) {
	config := DefaultPCMRouterConfig()
	router := NewPCMRouter(config, nil, nil, nil)
	defer router.Close()

	router.Reset()
	if router.State() != routerIdle {
		t.Error("should be idle after reset")
	}
}

func TestStreamReaderWriter(t *testing.T) {
	config := PCMRouterConfig{
		SampleRate: 16000,
		VADConfig: EnergyVADConfig{
			EnergyThreshold:   0.01,
			FrameDurationMs:   20,
			SilenceDurationMs: 60,
			SampleRate:        16000,
		},
		VADEnabled: true,
	}

	synthesisPCM := Int16SamplesToBytes(GenerateTone(16000, 220, 0.3, 100))
	mockSTT := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return &interfaces.TranscriptionResult{Text: "hi", Timestamp: time.Now()}, nil
		},
	}
	mockTTS := &MockSynthesizer{
		SynthesizeFunc: func(ctx context.Context, text string) (*interfaces.SynthesisResult, error) {
			return &interfaces.SynthesisResult{
				AudioData:  synthesisPCM,
				TextLength: len(text),
				Timestamp:  time.Now(),
			}, nil
		},
	}
	mockAgent := &MockAgentTextBridge{}

	router := NewPCMRouter(config, mockSTT, mockTTS, mockAgent)
	defer router.Close()

	sr := NewStreamReader(router)
	defer sr.Close()

	sw := NewStreamWriter(router)
	defer sw.Close()

	tonePCM := Int16SamplesToBytes(GenerateTone(16000, 440, 0.5, 100))
	_, _ = sw.Write(tonePCM)

	silencePCM := Int16SamplesToBytes(GenerateSilence(16000, 200))
	_, _ = sw.Write(silencePCM)

	time.Sleep(300 * time.Millisecond)

	buf := make([]byte, 1024)
	n, _ := sr.Read(buf)
	if n == 0 {
		t.Error("expected to read some output PCM data")
	}
}
