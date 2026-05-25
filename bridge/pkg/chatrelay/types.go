package chatrelay

import (
	"context"

	"github.com/armorclaw/bridge/internal/ai"
)

// MessageSender abstracts MatrixAdapter.SendMessageWithRetry to avoid circular imports.
// Signature matches: func (m *MatrixAdapter) SendMessageWithRetry(roomID, message, msgType string) (string, error)
type MessageSender interface {
	SendMessageWithRetry(roomID, message, msgType string) (string, error)
}

// AIChatFunc matches AIService.Chat signature:
// func (s *AIService) Chat(ctx context.Context, req ChatRequest, keyID string) (*ChatResponse, error)
type AIChatFunc func(ctx context.Context, req ai.ChatRequest, keyID string) (*ai.ChatResponse, error)
