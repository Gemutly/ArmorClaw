package voice

import (
	"context"

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
