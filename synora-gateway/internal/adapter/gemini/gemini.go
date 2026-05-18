package gemini

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/synora/synora-gateway/internal/adapter"
)

type GeminiAdapter struct{}

func NewGeminiAdapter() *GeminiAdapter {
	return &GeminiAdapter{}
}

// Request Types
type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerateContentRequest struct {
	Contents []Content `json:"contents"`
}

func (a *GeminiAdapter) ToRequest(req *adapter.UnifiedRequest) (interface{}, error) {
	contents := make([]Content, len(req.Messages))
	for i, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		} else if role == "system" {
			role = "user" // Gemini handling of system prompt varies, simplifying for MVP
		}
		contents[i] = Content{
			Role:  role,
			Parts: []Part{{Text: m.Content}},
		}
	}

	return GenerateContentRequest{
		Contents: contents,
	}, nil
}

func (a *GeminiAdapter) FromResponse(body []byte) (*adapter.UnifiedResponse, error) {
	var res struct {
		Candidates []struct {
			Content struct {
				Role  string `json:"role"`
				Parts []Part `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	choices := make([]adapter.Choice, len(res.Candidates))
	for i, c := range res.Candidates {
		text := ""
		if len(c.Content.Parts) > 0 {
			text = c.Content.Parts[0].Text
		}
		choices[i] = adapter.Choice{
			Index: i,
			Message: adapter.Message{
				Role:    "assistant",
				Content: text,
			},
			FinishReason: c.FinishReason,
		}
	}

	return &adapter.UnifiedResponse{
		ID:      uuid.New().String(),
		Model:   "gemini",
		Choices: choices,
		Usage: adapter.Usage{
			PromptTokens:     res.UsageMetadata.PromptTokenCount,
			CompletionTokens: res.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      res.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

func (a *GeminiAdapter) ConvertStreamChunk(line []byte) ([]byte, error) {
	// Gemini streaming typically returns a JSON array of candidates or individual JSON objects
	// For MVP, we need to convert this to OpenAI-compatible SSE format for the frontend
	// This is a complex mapping, using a simplified version for now.
	cleanLine := strings.TrimPrefix(strings.TrimSpace(string(line)), ",")
	if !strings.HasPrefix(cleanLine, "{") {
		return nil, nil
	}

	var chunk struct {
		Candidates []struct {
			Content struct {
				Parts []Part `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(cleanLine), &chunk); err != nil {
		return nil, nil
	}

	if len(chunk.Candidates) == 0 || len(chunk.Candidates[0].Content.Parts) == 0 {
		return nil, nil
	}

	text := chunk.Candidates[0].Content.Parts[0].Text
	
	// Create OpenAI-compatible chunk
	oaChunk := map[string]interface{}{
		"id":      "chatcmpl-" + uuid.New().String(),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   "gemini",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"delta": map[string]interface{}{
					"content": text,
				},
				"finish_reason": nil,
			},
		},
	}
	data, _ := json.Marshal(oaChunk)
	return append([]byte("data: "), append(data, []byte("\n\n")...)...), nil
}

func (a *GeminiAdapter) IsStreamEnd(line []byte) bool {
	// Gemini typically ends the JSON array or sends a specific finish field
	return bytes.Contains(line, []byte("usageMetadata"))
}

// ToUnifiedRequest converts Gemini request to UnifiedRequest
func ToUnifiedRequest(model string, req GenerateContentRequest, userID int) *adapter.UnifiedRequest {
	messages := make([]adapter.Message, len(req.Contents))
	for i, c := range req.Contents {
		role := c.Role
		if role == "model" {
			role = "assistant"
		}
		text := ""
		if len(c.Parts) > 0 {
			text = c.Parts[0].Text
		}
		messages[i] = adapter.Message{
			Role:    role,
			Content: text,
		}
	}

	return &adapter.UnifiedRequest{
		Model:     model,
		Messages:  messages,
		RequestID: uuid.New().String(),
		UserID:    userID,
		StartTime: time.Now(),
		// Stream is determined by the endpoint called
	}
}
