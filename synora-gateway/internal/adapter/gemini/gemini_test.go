package gemini

import (
	"testing"

	"github.com/synora/synora-gateway/internal/adapter"
)

func TestToUnifiedRequest(t *testing.T) {
	req := GenerateContentRequest{
		Contents: []Content{
			{Role: "user", Parts: []Part{{Text: "Hello Gemini"}}},
			{Role: "model", Parts: []Part{{Text: "Hello User"}}},
		},
	}

	unified := ToUnifiedRequest("gemini-pro", req, 456)

	if unified.Model != "gemini-pro" {
		t.Errorf("Expected model gemini-pro, got %s", unified.Model)
	}
	if unified.UserID != 456 {
		t.Errorf("Expected userID 456, got %d", unified.UserID)
	}
	if len(unified.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(unified.Messages))
	}
	if unified.Messages[0].Role != "user" || unified.Messages[0].Content != "Hello Gemini" {
		t.Errorf("First message conversion failed")
	}
	if unified.Messages[1].Role != "assistant" || unified.Messages[1].Content != "Hello User" {
		t.Errorf("Second message conversion failed")
	}
}

func TestGeminiAdapter_ToRequest(t *testing.T) {
	a := NewGeminiAdapter()
	unified := &adapter.UnifiedRequest{
		Messages: []adapter.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
			{Role: "system", Content: "System rule"},
		},
	}

	reqInterface, err := a.ToRequest(unified)
	if err != nil {
		t.Fatalf("ToRequest failed: %v", err)
	}

	req := reqInterface.(GenerateContentRequest)
	if len(req.Contents) != 3 {
		t.Errorf("Expected 3 contents, got %d", len(req.Contents))
	}
	if req.Contents[2].Role != "user" { // system is mapped to user for now in GeminiAdapter
		t.Errorf("System mapping failed, got %s", req.Contents[2].Role)
	}
}

func TestGeminiAdapter_FromResponse(t *testing.T) {
	a := NewGeminiAdapter()
	respJSON := `{
		"candidates": [
			{
				"content": {
					"role": "model",
					"parts": [{"text": "I am Gemini"}]
				},
				"finishReason": "STOP"
			}
		],
		"usageMetadata": {
			"promptTokenCount": 5,
			"candidatesTokenCount": 10,
			"totalTokenCount": 15
		}
	}`

	res, err := a.FromResponse([]byte(respJSON))
	if err != nil {
		t.Fatalf("FromResponse failed: %v", err)
	}

	if res.Choices[0].Message.Content != "I am Gemini" {
		t.Errorf("Content mismatch")
	}
	if res.Usage.PromptTokens != 5 || res.Usage.CompletionTokens != 10 {
		t.Errorf("Usage mismatch")
	}
}
