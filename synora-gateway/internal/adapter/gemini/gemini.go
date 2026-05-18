package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/synora/synora-gateway/internal/adapter"
)

type GeminiAdapter struct{}

func NewGeminiAdapter() *GeminiAdapter {
	return &GeminiAdapter{}
}

func (a *GeminiAdapter) ToRequest(req *adapter.UnifiedRequest) (interface{}, error) {
	// Simplified Gemini structure
	return map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"parts": []interface{}{
					map[string]interface{}{
						"text": req.Messages[len(req.Messages)-1].Content,
					},
				},
			},
		},
	}, nil
}

func (a *GeminiAdapter) FromResponse(body []byte) (*adapter.UnifiedResponse, error) {
	// Simplified Gemini response parsing
	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	content := ""
	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		content = res.Candidates[0].Content.Parts[0].Text
	}

	return &adapter.UnifiedResponse{
		Choices: []adapter.Choice{
			{
				Message: adapter.Message{
					Role:    "assistant",
					Content: content,
				},
			},
		},
	}, nil
}

func (a *GeminiAdapter) ConvertStreamChunk(line []byte) ([]byte, error) {
	return line, nil
}

func (a *GeminiAdapter) IsStreamEnd(line []byte) bool {
	return strings.Contains(string(line), "DONE") // Placeholder
}
