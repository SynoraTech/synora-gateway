package openai

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/synora/synora-gateway/internal/adapter"
)

type OpenAIAdapter struct{}

func NewOpenAIAdapter() *OpenAIAdapter {
	return &OpenAIAdapter{}
}

func (a *OpenAIAdapter) ToRequest(req *adapter.UnifiedRequest) (interface{}, error) {
	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
			Name:    m.Name,
		}
	}

	return openai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Stream:      req.Stream,
		Temperature: float32(req.Temperature),
		MaxTokens:   req.MaxTokens,
		TopP:        float32(req.TopP),
		Stop:        req.Stop,
	}, nil
}

func (a *OpenAIAdapter) FromResponse(body []byte) (*adapter.UnifiedResponse, error) {
	var res openai.ChatCompletionResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	choices := make([]adapter.Choice, len(res.Choices))
	for i, c := range res.Choices {
		choices[i] = adapter.Choice{
			Index: c.Index,
			Message: adapter.Message{
				Role:    c.Message.Role,
				Content: c.Message.Content,
			},
			FinishReason: string(c.FinishReason),
		}
	}

	return &adapter.UnifiedResponse{
		ID:      res.ID,
		Model:   res.Model,
		Choices: choices,
		Usage: adapter.Usage{
			PromptTokens:     res.Usage.PromptTokens,
			CompletionTokens: res.Usage.CompletionTokens,
			TotalTokens:      res.Usage.TotalTokens,
		},
	}, nil
}

func (a *OpenAIAdapter) ConvertStreamChunk(line []byte) ([]byte, error) {
	// OpenAI to OpenAI is a passthrough
	return line, nil
}

func (a *OpenAIAdapter) IsStreamEnd(line []byte) bool {
	return bytes.HasPrefix(line, []byte("data: [DONE]"))
}

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
