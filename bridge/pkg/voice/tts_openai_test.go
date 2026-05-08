package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOpenAITTSProvider_MissingAPIKey(t *testing.T) {
	_, err := NewOpenAITTSProvider(OpenAITTSConfig{})
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
	if !IsVoiceNotConfigured(err) {
		t.Errorf("expected ErrVoiceNotConfigured, got %v", err)
	}
}

func TestNewOpenAITTSProvider_Defaults(t *testing.T) {
	p, err := NewOpenAITTSProvider(OpenAITTSConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.model != "tts-1" {
		t.Errorf("expected model 'tts-1', got '%s'", p.model)
	}
	if p.voice != "alloy" {
		t.Errorf("expected voice 'alloy', got '%s'", p.voice)
	}
	if p.baseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default baseURL, got '%s'", p.baseURL)
	}
}

func TestNewOpenAITTSProvider_CustomConfig(t *testing.T) {
	p, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: "https://custom.api.com/v1",
		Model:   "tts-1-hd",
		Voice:   "nova",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.model != "tts-1-hd" {
		t.Errorf("expected model 'tts-1-hd', got '%s'", p.model)
	}
	if p.voice != "nova" {
		t.Errorf("expected voice 'nova', got '%s'", p.voice)
	}
	if p.baseURL != "https://custom.api.com/v1" {
		t.Errorf("expected custom baseURL, got '%s'", p.baseURL)
	}
}

func TestOpenAITTSProvider_Synthesize_Success(t *testing.T) {
	fakeAudio := []byte("fake-mp3-audio-data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/audio/speech" {
			t.Errorf("expected /audio/speech path, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer auth header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["model"] != "tts-1" {
			t.Errorf("expected model tts-1, got %s", body["model"])
		}
		if body["input"] != "hello world" {
			t.Errorf("expected input 'hello world', got '%s'", body["input"])
		}
		if body["voice"] != "alloy" {
			t.Errorf("expected voice alloy, got %s", body["voice"])
		}

		w.WriteHeader(http.StatusOK)
		w.Write(fakeAudio)
	}))
	defer server.Close()

	p, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := p.Synthesize(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.AudioData) != len(fakeAudio) {
		t.Errorf("expected %d bytes of audio, got %d", len(fakeAudio), len(result.AudioData))
	}
	if result.TextLength != 11 {
		t.Errorf("expected text length 11, got %d", result.TextLength)
	}
	if result.Latency == 0 {
		t.Error("expected non-zero latency")
	}
}

func TestOpenAITTSProvider_Synthesize_EmptyText(t *testing.T) {
	p, err := NewOpenAITTSProvider(OpenAITTSConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Synthesize(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}
}

func TestOpenAITTSProvider_Synthesize_RateLimit429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`))
	}))
	defer server.Close()

	p, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Synthesize(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for rate limit, got nil")
	}
	if !IsVoiceRateLimit(err) {
		t.Errorf("expected rate limit error (code -32008), got: %v", err)
	}

	var ve *VoiceError
	if !errorAsVoiceError(err, &ve) {
		t.Fatalf("expected *VoiceError, got %T", err)
	}
	if ve.Code != ErrVoiceRateLimitCode {
		t.Errorf("expected code %d, got %d", ErrVoiceRateLimitCode, ve.Code)
	}
}

func TestOpenAITTSProvider_Synthesize_QuotaExceeded402(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(`{"error":{"message":"You have exceeded your quota","type":"insufficient_quota"}}`))
	}))
	defer server.Close()

	p, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Synthesize(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for quota exceeded, got nil")
	}
	if !IsVoiceRateLimit(err) {
		t.Errorf("expected rate limit error (code -32008) for quota, got: %v", err)
	}
}

func TestOpenAITTSProvider_Synthesize_QuotaExceeded403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"message":"Billing limit exceeded","type":"billing_error"}}`))
	}))
	defer server.Close()

	p, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Synthesize(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for billing limit, got nil")
	}
	if !IsVoiceRateLimit(err) {
		t.Errorf("expected rate limit error (code -32008) for billing, got: %v", err)
	}
}

func TestOpenAITTSProvider_Synthesize_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"Internal server error"}}`))
	}))
	defer server.Close()

	p, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Synthesize(context.Background(), "test text")
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
	if IsVoiceRateLimit(err) {
		t.Errorf("should NOT be a rate limit error: %v", err)
	}
}

func TestOpenAITTSProvider_Synthesize_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer server.Close()

	p, err := NewOpenAITTSProvider(OpenAITTSConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = p.Synthesize(ctx, "test text")
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

func errorAsVoiceError(err error, ve **VoiceError) bool {
	return errorAs(err, ve)
}

func errorAs(err error, target interface{}) bool {
	switch v := target.(type) {
	case **VoiceError:
		if ve, ok := err.(*VoiceError); ok {
			*v = ve
			return true
		}
	}
	return false
}
