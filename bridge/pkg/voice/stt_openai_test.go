package voice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/armorclaw/bridge/pkg/interfaces"
)

func TestNewOpenAISTTProvider_MissingAPIKey(t *testing.T) {
	_, err := NewOpenAISTTProvider(OpenAISTTConfig{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !IsVoiceNotConfigured(err) {
		t.Errorf("expected ErrVoiceNotConfigured, got %v", err)
	}
}

func TestNewOpenAISTTProvider_Defaults(t *testing.T) {
	p, err := NewOpenAISTTProvider(OpenAISTTConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.baseURL != openAIDefaultBaseURL {
		t.Errorf("expected baseURL %s, got %s", openAIDefaultBaseURL, p.baseURL)
	}
	if p.model != openAIDefaultModel {
		t.Errorf("expected model %s, got %s", openAIDefaultModel, p.model)
	}
}

func TestOpenAISTTProvider_Transcribe_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/audio/transcriptions" {
			t.Errorf("expected /audio/transcriptions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("failed to parse multipart form: %v", err)
		}

		model := r.FormValue("model")
		if model != "whisper-1" {
			t.Errorf("expected model whisper-1, got %s", model)
		}

		resp := struct {
			Text     string  `json:"text"`
			Language string  `json:"language"`
			Duration float64 `json:"duration"`
		}{
			Text:     "hello world from whisper",
			Language: "en",
			Duration: 2.5,
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

	pcmData := make([]byte, 3200) // 100ms of 16kHz 16-bit mono PCM
	for i := range pcmData {
		pcmData[i] = byte(i % 256)
	}

	result, err := provider.Transcribe(context.Background(), pcmData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Text != "hello world from whisper" {
		t.Errorf("expected text 'hello world from whisper', got '%s'", result.Text)
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", result.Confidence)
	}
	if result.Duration != 2500*time.Millisecond {
		t.Errorf("expected duration 2500ms, got %v", result.Duration)
	}
	if result.WordCount != 4 {
		t.Errorf("expected word count 4, got %d", result.WordCount)
	}
	if result.Latency == 0 {
		t.Error("expected non-zero latency")
	}
}

func TestOpenAISTTProvider_Transcribe_RateLimit(t *testing.T) {
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

	pcmData := make([]byte, 3200)
	_, err = provider.Transcribe(context.Background(), pcmData)
	if err == nil {
		t.Fatal("expected error for rate limit")
	}

	var ve *VoiceError
	if !errors.As(err, &ve) {
		t.Fatalf("expected VoiceError, got %T: %v", err, err)
	}
	if ve.Code != ErrVoiceRateLimitCode {
		t.Errorf("expected error code %d, got %d", ErrVoiceRateLimitCode, ve.Code)
	}
	if ve.Message != "openai STT rate limit exceeded" {
		t.Errorf("unexpected message: %s", ve.Message)
	}
}

func TestOpenAISTTProvider_Transcribe_QuotaExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(`{"error":{"message":"Quota exceeded","type":"insufficient_quota"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAISTTProvider(OpenAISTTConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pcmData := make([]byte, 3200)
	_, err = provider.Transcribe(context.Background(), pcmData)
	if err == nil {
		t.Fatal("expected error for quota exceeded")
	}

	var ve *VoiceError
	if !errors.As(err, &ve) {
		t.Fatalf("expected VoiceError, got %T: %v", err, err)
	}
	if ve.Code != ErrVoiceRateLimitCode {
		t.Errorf("expected error code %d, got %d", ErrVoiceRateLimitCode, ve.Code)
	}
	if ve.Message != "openai STT quota exceeded" {
		t.Errorf("unexpected message: %s", ve.Message)
	}
}

func TestOpenAISTTProvider_Transcribe_EmptyAudio(t *testing.T) {
	provider, err := NewOpenAISTTProvider(OpenAISTTConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = provider.Transcribe(context.Background(), []byte{})
	if err == nil {
		t.Fatal("expected error for empty audio")
	}
	if err != interfaces.ErrEmptyAudioData {
		t.Errorf("expected ErrEmptyAudioData, got %v", err)
	}
}

func TestOpenAISTTProvider_Transcribe_ServerError(t *testing.T) {
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

	pcmData := make([]byte, 3200)
	_, err = provider.Transcribe(context.Background(), pcmData)
	if err == nil {
		t.Fatal("expected error for server error")
	}
	if errors.Is(err, &VoiceError{}) {
		t.Errorf("should not be a VoiceError for generic server errors")
	}
}

func TestOpenAISTTProvider_Transcribe_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	provider, err := NewOpenAISTTProvider(OpenAISTTConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	pcmData := make([]byte, 3200)
	_, err = provider.Transcribe(ctx, pcmData)
	if err == nil {
		t.Fatal("expected error for context cancellation")
	}
}

func TestPCMToWAV(t *testing.T) {
	pcmData := make([]byte, 16000*2) // 1 second of 16kHz 16-bit mono

	wavData, err := pcmToWAV(pcmData, 16000, 1, 16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(wavData) != 44+len(pcmData) {
		t.Errorf("expected WAV length %d, got %d", 44+len(pcmData), len(wavData))
	}

	if string(wavData[0:4]) != "RIFF" {
		t.Error("missing RIFF header")
	}
	if string(wavData[8:12]) != "WAVE" {
		t.Error("missing WAVE header")
	}
	if string(wavData[12:16]) != "fmt " {
		t.Error("missing fmt chunk")
	}
	if string(wavData[36:40]) != "data" {
		t.Error("missing data chunk")
	}
}

func TestPCMToWAV_EmptyData(t *testing.T) {
	_, err := pcmToWAV([]byte{}, 16000, 1, 16)
	if err == nil {
		t.Fatal("expected error for empty PCM data")
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"one two three four", 4},
	}
	for _, tt := range tests {
		got := countWords(tt.input)
		if got != tt.expected {
			t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestIsVoiceRateLimitError(t *testing.T) {
	err := &VoiceError{
		Code:    ErrVoiceRateLimitCode,
		Message: "rate limited",
	}
	if !isVoiceRateLimitError(err) {
		t.Error("expected rate limit error to be detected")
	}

	notRateLimit := &VoiceError{
		Code:    ErrVoiceNotConfiguredCode,
		Message: "not configured",
	}
	if isVoiceRateLimitError(notRateLimit) {
		t.Error("expected non-rate-limit error to not be detected")
	}
}

func isVoiceRateLimitError(err error) bool {
	var ve *VoiceError
	if errors.As(err, &ve) {
		return ve.Code == ErrVoiceRateLimitCode
	}
	return false
}
