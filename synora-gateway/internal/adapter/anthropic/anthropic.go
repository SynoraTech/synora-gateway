package anthropic

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/synora/synora-gateway/internal/adapter"
)

type AnthropicAdapter struct{}

func NewAnthropicAdapter() *AnthropicAdapter {
	return &AnthropicAdapter{}
}

// Response structures
type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (a *AnthropicAdapter) ToRequest(req *adapter.UnifiedRequest) (interface{}, error) {
	return UnifiedToAnthropic(req), nil
}

func (a *AnthropicAdapter) FromResponse(body []byte) (*adapter.UnifiedResponse, error) {
	var res MessageResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	var content string
	for _, b := range res.Content {
		content += b.Content
	}

	return &adapter.UnifiedResponse{
		ID:    res.ID,
		Model: res.Model,
		Choices: []adapter.Choice{
			{
				Index: 0,
				Message: adapter.Message{
					Role:    res.Role,
					Content: content,
				},
				FinishReason: res.StopReason,
			},
		},
		Usage: adapter.Usage{
			PromptTokens:     res.Usage.InputTokens,
			CompletionTokens: res.Usage.OutputTokens,
			TotalTokens:      res.Usage.InputTokens + res.Usage.OutputTokens,
		},
	}, nil
}

func (a *AnthropicAdapter) ConvertStreamChunk(line []byte) ([]byte, error) {
	// Anthropic SSE format is different from OpenAI.
	// For MVP, we might need a more sophisticated converter if the client expects OpenAI format.
	// If the client expects Anthropic format (Anthropic entry), we passthrough.
	// But Synora's goal is to be a bridge.
	
	// If the line is empty or doesn't start with "data: ", skip
	if len(line) == 0 || !bytes.HasPrefix(line, []byte("data: ")) {
		return line, nil
	}

	data := bytes.TrimPrefix(line, []byte("data: "))
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return line, nil
	}

	// This is where we'd convert Anthropic events to OpenAI data chunks if needed.
	// For now, let's keep it simple and just passthrough, 
	// but acknowledge that we need a better implementation for OpenAI-client -> Anthropic-upstream.
	return line, nil
}

func (a *AnthropicAdapter) IsStreamEnd(line []byte) bool {
	return strings.Contains(string(line), "message_stop")
}

// Anthropic specific models for /v1/messages
type MessageRequest struct {
	Model     string         `json:"model"`
	Messages  []ContentBlock `json:"messages"`
	System    string         `json:"system,omitempty"`
	MaxTokens int            `json:"max_tokens"`
	Stream    bool           `json:"stream,omitempty"`
}

type ContentBlock struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToUnifiedRequest converts Anthropic MessageRequest to UnifiedRequest
func ToUnifiedRequest(req MessageRequest, userID int) *adapter.UnifiedRequest {
	unifiedMessages := make([]adapter.Message, 0, len(req.Messages)+1)
	
	// Add system message if present
	if req.System != "" {
		unifiedMessages = append(unifiedMessages, adapter.Message{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, m := range req.Messages {
		unifiedMessages = append(unifiedMessages, adapter.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return &adapter.UnifiedRequest{
		Model:     req.Model,
		Messages:  unifiedMessages,
		Stream:    req.Stream,
		MaxTokens: req.MaxTokens,
		RequestID: uuid.New().String(),
		UserID:    userID,
		StartTime: time.Now(),
	}
}

// UnifiedToAnthropic converts UnifiedRequest back to Anthropic format (for upstream)
func UnifiedToAnthropic(req *adapter.UnifiedRequest) MessageRequest {
	var system string
	messages := make([]ContentBlock, 0, len(req.Messages))

	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		messages = append(messages, ContentBlock{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return MessageRequest{
		Model:     req.Model,
		Messages:  messages,
		System:    system,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}
}
