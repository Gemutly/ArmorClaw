package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/armorclaw/bridge/pkg/interfaces"
	"log/slog"
)

type Synthesizer interface {
	Synthesize(ctx context.Context, text string) (*interfaces.SynthesisResult, error)
}

type TTSService struct {
	client Synthesizer
	logger *slog.Logger
}

func NewTTSService(client Synthesizer) *TTSService {
	return &TTSService{
		client: client,
		logger: slog.Default(),
	}
}

func (s *TTSService) Synthesize(ctx context.Context, text string) (*interfaces.SynthesisResult, error) {
	result, err := s.client.Synthesize(ctx, text)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type OpenAITTSProvider struct {
	apiKey     string
	baseURL    string
	model      string
	voice      string
	httpClient *http.Client
}

type OpenAITTSConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Voice   string
}

func NewOpenAITTSProvider(cfg OpenAITTSConfig) (*OpenAITTSProvider, error) {
	if cfg.APIKey == "" {
		return nil, ErrVoiceNotConfigured
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "tts-1"
	}
	voice := cfg.Voice
	if voice == "" {
		voice = "alloy"
	}
	return &OpenAITTSProvider{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   model,
		voice:   voice,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

func NewOpenAITTSProviderFromEnv() (*OpenAITTSProvider, error) {
	apiKey := os.Getenv("OPEN_AI_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	return NewOpenAITTSProvider(OpenAITTSConfig{APIKey: apiKey})
}

func (p *OpenAITTSProvider) Synthesize(ctx context.Context, text string) (*interfaces.SynthesisResult, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text")
	}

	start := time.Now()

	reqBody := map[string]string{
		"model":          p.model,
		"input":          text,
		"voice":          p.voice,
		"response_format": "mp3",
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/audio/speech", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai TTS API error (%d): %s", resp.StatusCode, string(audioData))
	}

	return &interfaces.SynthesisResult{
		AudioData:  audioData,
		TextLength: len(text),
		Timestamp:  time.Now(),
		Latency:    time.Since(start),
	}, nil
}
