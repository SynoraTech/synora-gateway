package openai

import (
	"time"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/synora/synora-gateway/internal/adapter"
)

// ToUnifiedRequest converts OpenAI ChatCompletionRequest to UnifiedRequest
func ToUnifiedRequest(req openai.ChatCompletionRequest, userID int) *adapter.UnifiedRequest {
	unifiedMessages := make([]adapter.Message, len(req.Messages))
	for i, m := range req.Messages {
		unifiedMessages[i] = adapter.Message{
			Role:    m.Role,
			Content: m.Content,
			Name:    m.Name,
		}
	}

	return &adapter.UnifiedRequest{
		Model:       req.Model,
		Messages:    unifiedMessages,
		Stream:      req.Stream,
		Temperature: float64(req.Temperature),
		MaxTokens:   req.MaxTokens,
		TopP:        float64(req.TopP),
		RequestID:   uuid.New().String(),
		UserID:      userID,
		StartTime:   time.Now(),
	}
}

// FromUnifiedResponse converts UnifiedResponse to OpenAI ChatCompletionResponse
func FromUnifiedResponse(res *adapter.UnifiedResponse) openai.ChatCompletionResponse {
	choices := make([]openai.ChatCompletionChoice, len(res.Choices))
	for i, c := range res.Choices {
		choices[i] = openai.ChatCompletionChoice{
			Index: c.Index,
			Message: openai.ChatCompletionMessage{
				Role:    c.Message.Role,
				Content: c.Message.Content,
			},
			FinishReason: openai.FinishReason(c.FinishReason),
		}
	}

	return openai.ChatCompletionResponse{
		ID:      res.ID,
		Model:   res.Model,
		Choices: choices,
		Usage: openai.Usage{
			PromptTokens:     res.Usage.PromptTokens,
			CompletionTokens: res.Usage.CompletionTokens,
			TotalTokens:      res.Usage.TotalTokens,
		},
	}
}
