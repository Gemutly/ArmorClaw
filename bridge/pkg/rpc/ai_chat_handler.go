package rpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/armorclaw/bridge/internal/ai"
	"github.com/google/uuid"
)

func (s *Server) handleAIChat(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Messages    []ai.Message `json:"messages"`
		Model       string       `json:"model,omitempty"`
		Temperature float32      `json:"temperature,omitempty"`
		TopP        float32      `json:"top_p,omitempty"`
		MaxTokens   int          `json:"max_tokens,omitempty"`
		Stop        []string     `json:"stop,omitempty"`
		KeyID       string       `json:"key_id,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: ParseError, Message: err.Error()}
	}

	userID := "default"

	if s.aiService == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "AI service not configured"}
	}

	if !s.aiService.CheckRateLimit(userID) {
		return nil, &ErrorObj{Code: TooManyRequests, Message: "AI rate limit exceeded"}
	}

	// Acquire slot with context cancellation support (prevents goroutine leak)
	select {
	case s.aiSemaphore <- struct{}{}:
	case <-ctx.Done():
		return nil, &ErrorObj{
			Code:    RequestCancelled,
			Message: "request cancelled",
		}
	}

	defer func() { <-s.aiSemaphore }()

	// Timeout after semaphore acquisition (applies only to AI call)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	model := params.Model
	if model == "" {
		model = s.aiService.DefaultModel()
	}
	if model == "" {
		model = "gpt-4o"
	}

	chatReq := ai.ChatRequest{
		Model:       model,
		Messages:    params.Messages,
		Temperature: params.Temperature,
		TopP:        params.TopP,
		MaxTokens:   params.MaxTokens,
		Stop:        params.Stop,
		RequestID:   uuid.New().String(),
	}

	if err := s.aiService.ValidateRequest(chatReq); err != nil {
		return nil, &ErrorObj{Code: InvalidRequest, Message: err.Error()}
	}

	keyID := params.KeyID
	if keyID == "" {
		keyID = s.aiService.DefaultKeyID()
	}
	if keyID == "" {
		keyID = "openai-default"
	}

	start := time.Now()

	resp, err := s.aiService.Chat(ctx, chatReq, keyID)
	if err != nil {
		return nil, &ErrorObj{Code: ai.AIErrProviderError, Message: err.Error()}
	}

	latency := time.Since(start)
	slog.Info("ai.chat completed",
		"request_id", chatReq.RequestID,
		"model", model,
		"latency_ms", latency.Milliseconds(),
	)

	return resp, nil
}
