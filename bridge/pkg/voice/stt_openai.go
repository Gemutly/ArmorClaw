package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/armorclaw/bridge/pkg/interfaces"
)

const (
	openAIDefaultBaseURL = "https://api.openai.com/v1"
	openAIDefaultModel   = "whisper-1"
	openAITimeout        = 60 * time.Second
)

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
		baseURL = openAIDefaultBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = openAIDefaultModel
	}
	return &OpenAISTTProvider{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: openAITimeout,
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

	wavData, err := pcmToWAV(audioData, 16000, 1, 16)
	if err != nil {
		return nil, fmt.Errorf("converting PCM to WAV: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(wavData); err != nil {
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

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &VoiceError{
			Code:    ErrVoiceRateLimitCode,
			Message: "openai STT rate limit exceeded",
			Cause:   fmt.Errorf("HTTP 429: %s", string(body)),
		}
	}

	if resp.StatusCode == http.StatusPaymentRequired {
		return nil, &VoiceError{
			Code:    ErrVoiceRateLimitCode,
			Message: "openai STT quota exceeded",
			Cause:   fmt.Errorf("HTTP 402: %s", string(body)),
		}
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

	wordCount := countWords(result.Text)

	return &interfaces.TranscriptionResult{
		Text:       result.Text,
		Confidence: 1.0,
		Duration:   time.Duration(result.Duration * float64(time.Second)),
		WordCount:  wordCount,
		Timestamp:  time.Now(),
		Latency:    time.Since(start),
	}, nil
}

func countWords(text string) int {
	if text == "" {
		return 0
	}
	count := 0
	for _, r := range text {
		if r == ' ' {
			count++
		}
	}
	return count + 1
}

func pcmToWAV(pcmData []byte, sampleRate uint32, channels uint16, bitsPerSample uint16) ([]byte, error) {
	if len(pcmData) == 0 {
		return nil, fmt.Errorf("empty PCM data")
	}

	dataSize := uint32(len(pcmData))
	headerSize := uint32(44)

 wav := make([]byte, headerSize+dataSize)

	copy(wav[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(wav[4:8], headerSize+dataSize-8)
	copy(wav[8:12], []byte("WAVE"))
	copy(wav[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 1)
	binary.LittleEndian.PutUint16(wav[22:24], channels)
	binary.LittleEndian.PutUint32(wav[24:28], sampleRate)
	byteRate := sampleRate * uint32(channels) * uint32(bitsPerSample) / 8
	binary.LittleEndian.PutUint32(wav[28:32], byteRate)
	blockAlign := channels * bitsPerSample / 8
	binary.LittleEndian.PutUint16(wav[32:34], blockAlign)
	binary.LittleEndian.PutUint16(wav[34:36], bitsPerSample)
	copy(wav[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(wav[40:44], dataSize)

	copy(wav[headerSize:], pcmData)

	return wav, nil
}
