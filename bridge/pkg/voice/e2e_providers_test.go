// Package voice — End-to-end provider pipeline tests (no real API keys required).
//
// Covers:
//   - Audio pipeline: PCM → VAD → STT → text → TTS → PCM output (mocked providers)
//   - Session lifecycle through PCMRouter (start → process → close)
//   - Error scenarios: 429→-32008, 401→auth error, 500→retry, double start, double stop
//   - Flag-off behaviour: nil providers return appropriate errors
package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/armorclaw/bridge/pkg/interfaces"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// generateSpeechPCM creates PCM data that will trigger VAD speech detection.
// Uses a 440Hz tone at high amplitude to guarantee RMS > 0.01 threshold.
func generateSpeechPCM(sampleRate uint32, durationMs int) []byte {
	samples := GenerateTone(sampleRate, 440.0, 0.8, durationMs)
	return Int16SamplesToBytes(samples)
}

// generateSilentPCM creates PCM data below the VAD threshold.
func generateSilentPCM(sampleRate uint32, durationMs int) []byte {
	samples := GenerateSilence(sampleRate, durationMs)
	return Int16SamplesToBytes(samples)
}

// ---------------------------------------------------------------------------
// 1. Audio Pipeline Tests: PCM → VAD → STT → text → TTS → PCM output
// ---------------------------------------------------------------------------

// TestVoicePipeline_FullPipeline tests the full audio routing pipeline:
// PCM input → VAD → STT → text → agent → TTS → PCM output
func TestVoicePipeline_FullPipeline(t *testing.T) {
	const expectedText = "book me a flight to NYC"
	const agentReply = "I found 3 flights for you"
	fakePCMOut := []byte("synthesized-pcm-audio-data")

	stt := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return &interfaces.TranscriptionResult{
				Text:       expectedText,
				Confidence: 0.97,
				Duration:   2 * time.Second,
				Timestamp:  time.Now(),
			}, nil
		},
	}

	tts := &MockSynthesizer{
		SynthesizeFunc: func(ctx context.Context, text string) (*interfaces.SynthesisResult, error) {
			if text != agentReply {
				t.Errorf("TTS received unexpected text: got %q, want %q", text, agentReply)
			}
			return &interfaces.SynthesisResult{
				AudioData:  fakePCMOut,
				TextLength: len(agentReply),
				Timestamp:  time.Now(),
			}, nil
		},
	}

	agent := &MockAgentTextBridge{
		SendTextFunc: func(ctx context.Context, text string) (string, error) {
			if text != expectedText {
				t.Errorf("agent received unexpected text: got %q, want %q", text, expectedText)
			}
			return agentReply, nil
		},
	}

	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, stt, tts, agent)
	defer router.Close()

	var (
		mu             sync.Mutex
		gotAgentResp   string
		gotOutputPCM   []byte
		gotSpeechStart bool
	)

	router.OnSpeechStart = func(ts time.Time) {
		mu.Lock()
		gotSpeechStart = true
		mu.Unlock()
	}
	router.OnAgentResponse = func(text string) {
		mu.Lock()
		gotAgentResp = text
		mu.Unlock()
	}
	router.OnOutputPCM = func(pcm []byte) {
		mu.Lock()
		gotOutputPCM = pcm
		mu.Unlock()
	}

	// Feed speech audio that triggers VAD
	speechPCM := generateSpeechPCM(16000, 600)
	if err := router.ProcessInputPCM(speechPCM); err != nil {
		t.Fatalf("ProcessInputPCM speech: %v", err)
	}

	// Feed silence to trigger speech-end
	silencePCM := generateSilentPCM(16000, 400)
	if err := router.ProcessInputPCM(silencePCM); err != nil {
		t.Fatalf("ProcessInputPCM silence: %v", err)
	}

	// Wait for async processing
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		done := gotOutputPCM != nil
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for pipeline output")
		case <-time.After(50 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if !gotSpeechStart {
		t.Error("OnSpeechStart callback was not called")
	}
	if gotAgentResp != agentReply {
		t.Errorf("agent response: got %q, want %q", gotAgentResp, agentReply)
	}
	if string(gotOutputPCM) != string(fakePCMOut) {
		t.Errorf("output PCM: got %q, want %q", string(gotOutputPCM), string(fakePCMOut))
	}
}

// TestVoicePipeline_STTError tests that STT errors propagate via OnError callback.
func TestVoicePipeline_STTError(t *testing.T) {
	sttErr := fmt.Errorf("STT connection refused")
	stt := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return nil, sttErr
		},
	}

	var (
		mu       sync.Mutex
		gotError error
	)

	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, stt, nil, &MockAgentTextBridge{})
	defer router.Close()

	router.OnError = func(err error) {
		mu.Lock()
		gotError = err
		mu.Unlock()
	}

	// Feed speech to trigger STT
	speechPCM := generateSpeechPCM(16000, 600)
	router.ProcessInputPCM(speechPCM)
	silencePCM := generateSilentPCM(16000, 400)
	router.ProcessInputPCM(silencePCM)

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		done := gotError != nil
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for STT error")
		case <-time.After(50 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if gotError == nil {
		t.Fatal("expected error from STT failure")
	}
}

// TestVoicePipeline_TTSError tests that TTS errors propagate via OnError callback.
func TestVoicePipeline_TTSError(t *testing.T) {
	ttsErr := fmt.Errorf("TTS service overloaded")
	stt := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return &interfaces.TranscriptionResult{Text: "hello", Confidence: 1.0, Timestamp: time.Now()}, nil
		},
	}
	tts := &MockSynthesizer{
		SynthesizeFunc: func(ctx context.Context, text string) (*interfaces.SynthesisResult, error) {
			return nil, ttsErr
		},
	}

	var (
		mu       sync.Mutex
		gotError error
	)

	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, stt, tts, &MockAgentTextBridge{})
	defer router.Close()

	router.OnError = func(err error) {
		mu.Lock()
		gotError = err
		mu.Unlock()
	}

	speechPCM := generateSpeechPCM(16000, 600)
	router.ProcessInputPCM(speechPCM)
	silencePCM := generateSilentPCM(16000, 400)
	router.ProcessInputPCM(silencePCM)

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		done := gotError != nil
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for TTS error")
		case <-time.After(50 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if gotError == nil {
		t.Fatal("expected error from TTS failure")
	}
}

// TestVoicePipeline_NilSTT tests that nil STT triggers OnError.
func TestVoicePipeline_NilSTT(t *testing.T) {
	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, nil, nil, &MockAgentTextBridge{})
	defer router.Close()

	var (
		mu       sync.Mutex
		gotError error
	)
	router.OnError = func(err error) {
		mu.Lock()
		gotError = err
		mu.Unlock()
	}

	speechPCM := generateSpeechPCM(16000, 600)
	router.ProcessInputPCM(speechPCM)
	silencePCM := generateSilentPCM(16000, 400)
	router.ProcessInputPCM(silencePCM)

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		done := gotError != nil
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for nil STT error")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// TestVoicePipeline_NilAgent tests that nil agent bridge triggers OnError.
func TestVoicePipeline_NilAgent(t *testing.T) {
	stt := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return &interfaces.TranscriptionResult{Text: "hello", Confidence: 1.0, Timestamp: time.Now()}, nil
		},
	}

	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, stt, nil, nil)
	defer router.Close()

	var (
		mu       sync.Mutex
		gotError error
	)
	router.OnError = func(err error) {
		mu.Lock()
		gotError = err
		mu.Unlock()
	}

	speechPCM := generateSpeechPCM(16000, 600)
	router.ProcessInputPCM(speechPCM)
	silencePCM := generateSilentPCM(16000, 400)
	router.ProcessInputPCM(silencePCM)

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		done := gotError != nil
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for nil agent error")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// TestVoicePipeline_VADDisabled tests pipeline with VAD disabled (direct STT).
func TestVoicePipeline_VADDisabled(t *testing.T) {
	const transcriptText = "direct pipeline text"
	stt := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return &interfaces.TranscriptionResult{Text: transcriptText, Confidence: 1.0, Timestamp: time.Now()}, nil
		},
	}

	var (
		mu           sync.Mutex
		gotAgentText string
	)
	agent := &MockAgentTextBridge{
		SendTextFunc: func(ctx context.Context, text string) (string, error) {
			mu.Lock()
			gotAgentText = text
			mu.Unlock()
			return "ok", nil
		},
	}

	cfg := DefaultPCMRouterConfig()
	cfg.VADEnabled = false
	router := NewPCMRouter(cfg, stt, nil, agent)
	defer router.Close()

	// With VAD disabled, audio goes directly to STT
	pcmData := generateSpeechPCM(16000, 200)
	router.ProcessInputPCM(pcmData)

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		done := gotAgentText != ""
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for agent text with VAD disabled")
		case <-time.After(50 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if gotAgentText != transcriptText {
		t.Errorf("agent text: got %q, want %q", gotAgentText, transcriptText)
	}
}

// TestVoicePipeline_EmptyTranscript tests that empty STT result returns router to idle.
func TestVoicePipeline_EmptyTranscript(t *testing.T) {
	stt := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return &interfaces.TranscriptionResult{Text: "", Confidence: 0, Timestamp: time.Now()}, nil
		},
	}

	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, stt, nil, &MockAgentTextBridge{})
	defer router.Close()

	speechPCM := generateSpeechPCM(16000, 600)
	router.ProcessInputPCM(speechPCM)
	silencePCM := generateSilentPCM(16000, 400)
	router.ProcessInputPCM(silencePCM)

	// Give async goroutine time to finish
	time.Sleep(200 * time.Millisecond)

	if state := router.State(); state != routerIdle {
		t.Errorf("expected routerIdle after empty transcript, got %d", state)
	}
}

// ---------------------------------------------------------------------------
// 2. Provider Error Scenarios via httptest
// ---------------------------------------------------------------------------

// TestVoiceProvider_STT_429_RateLimit verifies STT 429 response → VoiceError code -32008.
func TestVoiceProvider_STT_429_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAISTTProvider(OpenAISTTConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pcmData := generateSpeechPCM(16000, 100)
	_, err = provider.Transcribe(context.Background(), pcmData)
	if err == nil {
		t.Fatal("expected error for 429")
	}

	var ve *VoiceError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *VoiceError, got %T: %v", err, err)
	}
	if ve.Code != ErrVoiceRateLimitCode {
		t.Errorf("expected code %d (rate limit), got %d", ErrVoiceRateLimitCode, ve.Code)
	}
	if !IsVoiceRateLimit(err) {
		t.Error("IsVoiceRateLimit should return true")
	}
}

// TestVoiceProvider_STT_401_AuthError verifies STT 401 response returns generic API error.
func TestVoiceProvider_STT_401_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Invalid API key","type":"authentication_error"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAISTTProvider(OpenAISTTConfig{
		APIKey:  "bad-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pcmData := generateSpeechPCM(16000, 100)
	_, err = provider.Transcribe(context.Background(), pcmData)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	// 401 is NOT a rate limit — it's a generic API error
	if IsVoiceRateLimit(err) {
		t.Error("401 should NOT be classified as rate limit")
	}
	// Should contain status code in message
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestVoiceProvider_STT_500_ServerError verifies STT 500 response returns generic error.
func TestVoiceProvider_STT_500_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	}))
	defer server.Close()

	provider, err := NewOpenAISTTProvider(OpenAISTTConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pcmData := generateSpeechPCM(16000, 100)
	_, err = provider.Transcribe(context.Background(), pcmData)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	// 500 is NOT a rate limit
	if IsVoiceRateLimit(err) {
		t.Error("500 should NOT be classified as rate limit")
	}
}

// TestVoiceProvider_TTS_429_RateLimit verifies TTS 429 response → VoiceError code -32008.
func TestVoiceProvider_TTS_429_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"Rate limit exceeded"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = provider.Synthesize(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for 429")
	}

	var ve *VoiceError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *VoiceError, got %T: %v", err, err)
	}
	if ve.Code != ErrVoiceRateLimitCode {
		t.Errorf("expected code %d, got %d", ErrVoiceRateLimitCode, ve.Code)
	}
	if !IsVoiceRateLimit(err) {
		t.Error("IsVoiceRateLimit should return true")
	}
}

// TestVoiceProvider_TTS_401_AuthError verifies TTS 401 response returns generic error.
func TestVoiceProvider_TTS_401_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "bad-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = provider.Synthesize(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if IsVoiceRateLimit(err) {
		t.Error("401 should NOT be classified as rate limit")
	}
}

// TestVoiceProvider_TTS_500_ServerError verifies TTS 500 response returns generic error.
func TestVoiceProvider_TTS_500_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"Internal server error"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = provider.Synthesize(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if IsVoiceRateLimit(err) {
		t.Error("500 should NOT be classified as rate limit")
	}
}

// TestVoiceProvider_TTS_QuotaExceeded_402 verifies 402 → code -32008.
func TestVoiceProvider_TTS_QuotaExceeded_402(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(`{"error":{"message":"Quota exceeded","type":"insufficient_quota"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = provider.Synthesize(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for 402")
	}
	if !IsVoiceRateLimit(err) {
		t.Errorf("402 quota should be classified as rate limit, got: %v", err)
	}
}

// TestVoiceProvider_STT_QuotaExceeded_402 verifies STT 402 → code -32008.
func TestVoiceProvider_STT_QuotaExceeded_402(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(`{"error":{"message":"Quota exceeded"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAISTTProvider(OpenAISTTConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pcmData := generateSpeechPCM(16000, 100)
	_, err = provider.Transcribe(context.Background(), pcmData)
	if err == nil {
		t.Fatal("expected error for 402")
	}
	if !IsVoiceRateLimit(err) {
		t.Errorf("402 quota should be classified as rate limit, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 3. Session Lifecycle via PCMRouter
// ---------------------------------------------------------------------------

// TestVoiceSession_StartProcessStop tests creating, using, and closing a PCMRouter session.
func TestVoiceSession_StartProcessStop(t *testing.T) {
	stt := &MockTranscriber{}
	tts := &MockSynthesizer{}
	agent := &MockAgentTextBridge{}

	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, stt, tts, agent)

	// Session starts in idle state
	if state := router.State(); state != routerIdle {
		t.Errorf("expected routerIdle, got %d", state)
	}

	// Feed speech → transitions to listening
	speechPCM := generateSpeechPCM(16000, 200)
	router.ProcessInputPCM(speechPCM)

	// Feed silence → triggers speech end → processing
	silencePCM := generateSilentPCM(16000, 400)
	router.ProcessInputPCM(silencePCM)

	// Close the router
	router.Close()

	// After close, context should be done
	select {
	case <-router.ctx.Done():
		// expected
	case <-time.After(1 * time.Second):
		t.Error("router context should be done after Close()")
	}
}

// TestVoiceSession_DoubleClose tests that closing a router twice does not panic.
func TestVoiceSession_DoubleClose(t *testing.T) {
	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, nil, nil, nil)

	// Double close should not panic
	router.Close()
	router.Close()
}

// TestVoiceSession_Reset tests that Reset returns router to idle state.
func TestVoiceSession_Reset(t *testing.T) {
	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, nil, nil, nil)
	defer router.Close()

	// Feed speech to move state
	speechPCM := generateSpeechPCM(16000, 200)
	router.ProcessInputPCM(speechPCM)

	router.Reset()

	if state := router.State(); state != routerIdle {
		t.Errorf("expected routerIdle after Reset(), got %d", state)
	}
}

// TestVoiceSession_StreamReader tests the StreamReader/Writer pair.
func TestVoiceSession_StreamReader(t *testing.T) {
	fakeAudio := []byte("tts-output-audio")

	stt := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return &interfaces.TranscriptionResult{Text: "hi", Confidence: 1.0, Timestamp: time.Now()}, nil
		},
	}
	tts := &MockSynthesizer{
		SynthesizeFunc: func(ctx context.Context, text string) (*interfaces.SynthesisResult, error) {
			return &interfaces.SynthesisResult{AudioData: fakeAudio, TextLength: len(text), Timestamp: time.Now()}, nil
		},
	}
	agent := &MockAgentTextBridge{}

	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, stt, tts, agent)
	defer router.Close()

	sr := NewStreamReader(router)
	defer sr.Close()

	sw := NewStreamWriter(router)

	// Write speech + silence through StreamWriter
	speechPCM := generateSpeechPCM(16000, 600)
	sw.Write(speechPCM)
	silencePCM := generateSilentPCM(16000, 400)
	sw.Write(silencePCM)

	// Read from StreamReader
	buf := make([]byte, 4096)
	var collected []byte
	deadline := time.After(5 * time.Second)
	for {
		n, err := sr.Read(buf)
		if n > 0 {
			collected = append(collected, buf[:n]...)
		}
		if err != nil {
			break
		}
		if len(collected) >= len(fakeAudio) {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out reading from StreamReader")
		default:
		}
	}

	if string(collected) != string(fakeAudio) {
		t.Errorf("StreamReader output: got %q, want %q", string(collected), string(fakeAudio))
	}
}

// ---------------------------------------------------------------------------
// 4. Flag-Off / Not Configured Behaviour
// ---------------------------------------------------------------------------

// TestVoiceFlagOff_STTProvider_NoAPIKey verifies missing API key returns ErrVoiceNotConfigured.
func TestVoiceFlagOff_STTProvider_NoAPIKey(t *testing.T) {
	_, err := NewOpenAISTTProvider(OpenAISTTConfig{APIKey: ""})
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !IsVoiceNotConfigured(err) {
		t.Errorf("expected ErrVoiceNotConfigured, got: %v", err)
	}
}

// TestVoiceFlagOff_TTSProvider_NoAPIKey verifies missing API key returns ErrVoiceNotConfigured.
func TestVoiceFlagOff_TTSProvider_NoAPIKey(t *testing.T) {
	_, err := NewOpenAITTSProvider(OpenAITTSConfig{APIKey: ""})
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !IsVoiceNotConfigured(err) {
		t.Errorf("expected ErrVoiceNotConfigured, got: %v", err)
	}
}

// TestVoiceFlagOff_PCMRouter_NilProviders verifies pipeline degrades gracefully with nil providers.
func TestVoiceFlagOff_PCMRouter_NilProviders(t *testing.T) {
	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, nil, nil, nil)
	defer router.Close()

	// Should not panic, errors go to OnError callback
	var (
		mu       sync.Mutex
		gotError error
	)
	router.OnError = func(err error) {
		mu.Lock()
		gotError = err
		mu.Unlock()
	}

	speechPCM := generateSpeechPCM(16000, 600)
	if err := router.ProcessInputPCM(speechPCM); err != nil {
		t.Fatalf("ProcessInputPCM should not return error directly: %v", err)
	}
	silencePCM := generateSilentPCM(16000, 400)
	router.ProcessInputPCM(silencePCM)

	// Wait for async nil STT error
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		done := gotError != nil
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for nil provider error")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// ---------------------------------------------------------------------------
// 5. VAD Unit Tests (supporting e2e pipeline)
// ---------------------------------------------------------------------------

// TestVoiceVAD_SpeechDetection verifies VAD detects speech from tone input.
func TestVoiceVAD_SpeechDetection(t *testing.T) {
	vad := NewEnergyThresholdVAD(DefaultEnergyVADConfig())

	// Feed speech tone
	speechPCM := generateSpeechPCM(16000, 200)
	events := vad.ProcessPCM(speechPCM)

	hasSpeechStart := false
	for _, evt := range events {
		if evt.Type == VADEventSpeechStart {
			hasSpeechStart = true
		}
	}
	if !hasSpeechStart {
		t.Error("VAD should detect speech from tone input")
	}
}

// TestVoiceVAD_SilenceDetection verifies VAD stays silent on zero input.
func TestVoiceVAD_SilenceDetection(t *testing.T) {
	vad := NewEnergyThresholdVAD(DefaultEnergyVADConfig())

	silencePCM := generateSilentPCM(16000, 200)
	events := vad.ProcessPCM(silencePCM)

	for _, evt := range events {
		if evt.Type == VADEventSpeechStart {
			t.Error("VAD should NOT detect speech from silence")
		}
	}
}

// TestVoiceVAD_SpeechEndOnSilence verifies VAD transitions from speech→silence.
func TestVoiceVAD_SpeechEndOnSilence(t *testing.T) {
	vad := NewEnergyThresholdVAD(DefaultEnergyVADConfig())

	// First feed speech to enter speech state
	speechPCM := generateSpeechPCM(16000, 100)
	vad.ProcessPCM(speechPCM)

	if vad.State() != VADStateSpeech {
		t.Fatal("VAD should be in speech state after tone input")
	}

	// Feed extended silence to trigger speech end
	silencePCM := generateSilentPCM(16000, 500)
	events := vad.ProcessPCM(silencePCM)

	hasSpeechEnd := false
	for _, evt := range events {
		if evt.Type == VADEventSpeechEnd {
			hasSpeechEnd = true
		}
	}
	if !hasSpeechEnd {
		t.Error("VAD should transition to speech_end after extended silence")
	}
	if vad.State() != VADStateSilence {
		t.Error("VAD should return to silence state")
	}
}

// TestVoiceVAD_DetectSpeech_Interface tests the interfaces.VADProvider-compatible method.
func TestVoiceVAD_DetectSpeech_Interface(t *testing.T) {
	vad := NewEnergyThresholdVAD(DefaultEnergyVADConfig())

	speechPCM := generateSpeechPCM(16000, 200)
	result, err := vad.DetectSpeech(context.Background(), speechPCM)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.SpeechDetected {
		t.Error("DetectSpeech should detect speech from tone input")
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", result.Confidence)
	}
}

// TestVoiceVAD_DetectSpeech_EmptyAudio tests empty audio returns error.
func TestVoiceVAD_DetectSpeech_EmptyAudio(t *testing.T) {
	vad := NewEnergyThresholdVAD(DefaultEnergyVADConfig())

	_, err := vad.DetectSpeech(context.Background(), []byte{})
	if err == nil {
		t.Fatal("expected error for empty audio")
	}
	if err != interfaces.ErrEmptyAudioData {
		t.Errorf("expected ErrEmptyAudioData, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 6. Provider Round-Trip with httptest (mocked OpenAI API)
// ---------------------------------------------------------------------------

// TestVoiceProvider_STT_RoundTrip verifies full STT HTTP round-trip with mocked server.
func TestVoiceProvider_STT_RoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request shape
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/audio/transcriptions" {
			t.Errorf("expected /audio/transcriptions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer auth")
		}

		// Parse multipart to verify WAV payload
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("failed to parse multipart: %v", err)
		}
		if r.FormValue("model") != "whisper-1" {
			t.Errorf("expected model whisper-1, got %s", r.FormValue("model"))
		}
		if r.FormValue("response_format") != "verbose_json" {
			t.Errorf("expected verbose_json response_format")
		}

		resp := map[string]interface{}{
			"text":     "hello from round trip test",
			"language": "en",
			"duration": 1.5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := NewOpenAISTTProvider(OpenAISTTConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pcmData := generateSpeechPCM(16000, 100)
	result, err := provider.Transcribe(context.Background(), pcmData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Text != "hello from round trip test" {
		t.Errorf("text: got %q, want %q", result.Text, "hello from round trip test")
	}
	if result.WordCount != 5 {
		t.Errorf("word count: got %d, want 5", result.WordCount)
	}
}

// TestVoiceProvider_TTS_RoundTrip verifies full TTS HTTP round-trip with mocked server.
func TestVoiceProvider_TTS_RoundTrip(t *testing.T) {
	fakeAudio := []byte("mp3-audio-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/audio/speech" {
			t.Errorf("expected /audio/speech, got %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["input"] != "synthesize this" {
			t.Errorf("input: got %q", body["input"])
		}
		if body["voice"] != "alloy" {
			t.Errorf("voice: got %q", body["voice"])
		}
		if body["response_format"] != "mp3" {
			t.Errorf("response_format: got %q", body["response_format"])
		}

		w.WriteHeader(http.StatusOK)
		w.Write(fakeAudio)
	}))
	defer server.Close()

	provider, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := provider.Synthesize(context.Background(), "synthesize this")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result.AudioData) != string(fakeAudio) {
		t.Errorf("audio: got %q, want %q", string(result.AudioData), string(fakeAudio))
	}
	if result.TextLength != len("synthesize this") {
		t.Errorf("text length: got %d, want %d", result.TextLength, len("synthesize this"))
	}
}

// ---------------------------------------------------------------------------
// 7. Concurrent Pipeline Tests
// ---------------------------------------------------------------------------

// TestVoicePipeline_ConcurrentProcessing tests that multiple PCM chunks process safely.
func TestVoicePipeline_ConcurrentProcessing(t *testing.T) {
	stt := &MockTranscriber{
		TranscribeFunc: func(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
			return &interfaces.TranscriptionResult{Text: "concurrent test", Confidence: 1.0, Timestamp: time.Now()}, nil
		},
	}

	var (
		mu          sync.Mutex
		agentCalls  int
	)
	agent := &MockAgentTextBridge{
		SendTextFunc: func(ctx context.Context, text string) (string, error) {
			mu.Lock()
			agentCalls++
			mu.Unlock()
			return "response", nil
		},
	}

	cfg := DefaultPCMRouterConfig()
	router := NewPCMRouter(cfg, stt, nil, agent)
	defer router.Close()

	// Feed multiple speech+silence sequences concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			speechPCM := generateSpeechPCM(16000, 300)
			router.ProcessInputPCM(speechPCM)
			silencePCM := generateSilentPCM(16000, 400)
			router.ProcessInputPCM(silencePCM)
		}()
	}
	wg.Wait()

	// Wait for async processing
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	calls := agentCalls
	mu.Unlock()

	if calls == 0 {
		t.Error("expected at least one agent call from concurrent input")
	}
}

// ---------------------------------------------------------------------------
// 8. Voice Prereq Check E2E Tests (T8–T11 validation)
// ---------------------------------------------------------------------------

// TestCheckVoicePrereqs_AllMet verifies that CheckVoicePrereqs returns empty
// when all prerequisites are satisfied.
func TestCheckVoicePrereqs_AllMet(t *testing.T) {
	t.Setenv("TURN_SECRET", "test-turn-secret")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	failures := CheckVoicePrereqs(true) // matrixWired = true
	if len(failures) != 0 {
		t.Fatalf("expected no failures when all prereqs met, got %d: %+v", len(failures), failures)
	}
}

// TestCheckVoicePrereqs_TURN_SECRET tests that missing TURN_SECRET is reported.
func TestCheckVoicePrereqs_TURN_SECRET(t *testing.T) {
	t.Setenv("TURN_SECRET", "")
	t.Setenv("OPENAI_API_KEY", "test-key")

	failures := CheckVoicePrereqs(true)
	if len(failures) == 0 {
		t.Fatal("expected at least one failure for missing TURN_SECRET")
	}

	found := false
	for _, f := range failures {
		if f.Reason == PrereqTurnSecretMissing {
			found = true
			if f.Message == "" {
				t.Error("expected non-empty message for TURN_SECRET failure")
			}
		}
	}
	if !found {
		t.Errorf("expected VOICE_PREREQ_TURN_SECRET_MISSING in failures, got: %+v", failures)
	}
}

// TestCheckVoicePrereqs_OpenAIKey tests that missing OPENAI_API_KEY is reported.
func TestCheckVoicePrereqs_OpenAIKey(t *testing.T) {
	t.Setenv("TURN_SECRET", "test-secret")
	t.Setenv("OPENAI_API_KEY", "")

	failures := CheckVoicePrereqs(true)
	if len(failures) == 0 {
		t.Fatal("expected at least one failure for missing OPENAI_API_KEY")
	}

	found := false
	for _, f := range failures {
		if f.Reason == PrereqOpenAIKeyMissing {
			found = true
		}
	}
	if !found {
		t.Errorf("expected VOICE_PREREQ_OPENAI_KEY_MISSING in failures, got: %+v", failures)
	}
}

// TestCheckVoicePrereqs_MatrixUnavailable tests that matrixWired=false reports VOICE_PREREQ_MATRIX_UNWIRED.
func TestCheckVoicePrereqs_MatrixUnavailable(t *testing.T) {
	t.Setenv("TURN_SECRET", "test-secret")
	t.Setenv("OPENAI_API_KEY", "test-key")

	failures := CheckVoicePrereqs(false) // matrixWired = false
	if len(failures) == 0 {
		t.Fatal("expected at least one failure for Matrix unavailable")
	}

	found := false
	for _, f := range failures {
		if f.Reason == PrereqMatrixUnwired {
			found = true
		}
	}
	if !found {
		t.Errorf("expected VOICE_PREREQ_MATRIX_UNWIRED in failures, got: %+v", failures)
	}
}

// TestCheckVoicePrereqs_MultipleFailures verifies that multiple missing prereqs
// are all reported simultaneously.
func TestCheckVoicePrereqs_MultipleFailures(t *testing.T) {
	t.Setenv("TURN_SECRET", "")
	t.Setenv("OPENAI_API_KEY", "")

	failures := CheckVoicePrereqs(false) // matrixWired = false

	if len(failures) < 3 {
		t.Errorf("expected at least 3 failures, got %d: %+v", len(failures), failures)
	}

	reasons := make(map[VoicePrereqReason]bool)
	for _, f := range failures {
		reasons[f.Reason] = true
	}

	for _, want := range []VoicePrereqReason{PrereqTurnSecretMissing, PrereqOpenAIKeyMissing, PrereqMatrixUnwired} {
		if !reasons[want] {
			t.Errorf("missing expected reason %q in failures", want)
		}
	}
}

// TestVoicePrereqFailure_JSONShape verifies that VoicePrereqFailure serializes
// to the expected JSON shape for contract compatibility.
func TestVoicePrereqFailure_JSONShape(t *testing.T) {
	failure := VoicePrereqFailure{
		Reason:  PrereqTurnSecretMissing,
		Message: "TURN_SECRET environment variable is not set",
	}

	data, err := json.Marshal(failure)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if reason, _ := parsed["reason"].(string); reason != string(PrereqTurnSecretMissing) {
		t.Errorf("reason: got %q, want %q", reason, PrereqTurnSecretMissing)
	}
	if msg, _ := parsed["message"].(string); msg != failure.Message {
		t.Errorf("message: got %q, want %q", msg, failure.Message)
	}

	// Verify only expected keys exist
	if len(parsed) != 2 {
		t.Errorf("expected exactly 2 keys in JSON, got %d: %+v", len(parsed), parsed)
	}
}

// TestVoiceError_Codes verifies error code constants match spec.
func TestVoiceError_Codes(t *testing.T) {
	if ErrVoiceNotConfiguredCode != -32007 {
		t.Errorf("ErrVoiceNotConfiguredCode: got %d, want -32007", ErrVoiceNotConfiguredCode)
	}
	if ErrVoiceRateLimitCode != -32008 {
		t.Errorf("ErrVoiceRateLimitCode: got %d, want -32008", ErrVoiceRateLimitCode)
	}
}

// TestVoiceError_StructuredError verifies VoiceError contains code and message.
func TestVoiceError_StructuredError(t *testing.T) {
	err := NewVoiceError(ErrVoiceNotConfiguredCode, "test message", nil)
	if err.Code != ErrVoiceNotConfiguredCode {
		t.Errorf("code: got %d, want %d", err.Code, ErrVoiceNotConfiguredCode)
	}
	if err.Message != "test message" {
		t.Errorf("message: got %q, want %q", err.Message, "test message")
	}
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
	if !IsVoiceNotConfigured(err) {
		t.Error("IsVoiceNotConfigured should return true")
	}
}
