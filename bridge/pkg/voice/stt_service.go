package voice

import (
	"context"

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
