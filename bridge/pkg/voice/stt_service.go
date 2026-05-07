package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/armorclaw/bridge/pkg/interfaces"
	"log/slog"
)

type Transcriber interface {
	Transcribe(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error)
}

type STTService struct {
	client Transcriber
	logger *slog.Logger
}

func NewSTTService(client Transcriber) *STTService {
	return &STTService{
		client: client,
		logger: slog.Default(),
	}
}

func (s *STTService) Transcribe(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
	result, err := s.client.Transcribe(ctx, audioData)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type OpenAISTTProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

type OpenAISTTConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

func NewOpenAISTTProvider(cfg OpenAISTTConfig) (*OpenAISTTProvider, error) {
	if cfg.APIKey == "" {
		return nil, ErrVoiceNotConfigured
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "whisper-1"
	}
	return &OpenAISTTProvider{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func NewOpenAISTTProviderFromEnv() (*OpenAISTTProvider, error) {
	apiKey := os.Getenv("OPEN_AI_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	return NewOpenAISTTProvider(OpenAISTTConfig{APIKey: apiKey})
}

func (p *OpenAISTTProvider) Transcribe(ctx context.Context, audioData []byte) (*interfaces.TranscriptionResult, error) {
	if len(audioData) == 0 {
		return nil, interfaces.ErrEmptyAudioData
	}

	start := time.Now()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return nil, fmt.Errorf("writing audio data: %w", err)
	}
	if err := writer.WriteField("model", p.model); err != nil {
		return nil, fmt.Errorf("writing model field: %w", err)
	}
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("writing response_format field: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai STT API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text     string  `json:"text"`
		Language string  `json:"language"`
		Duration float64 `json:"duration"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	wordCount := 0
	for _, r := range result.Text {
		if r == ' ' {
			wordCount++
		}
	}
	if len(result.Text) > 0 {
		wordCount++
	}

	return &interfaces.TranscriptionResult{
		Text:       result.Text,
		Confidence: 1.0,
		Duration:   time.Duration(result.Duration * float64(time.Second)),
		WordCount:  wordCount,
		Timestamp:  time.Now(),
		Latency:    time.Since(start),
	}, nil
}
