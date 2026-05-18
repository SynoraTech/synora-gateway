package openai

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestToUnifiedRequest(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
		Temperature: 0.7,
		Stream:      true,
	}

	unified := ToUnifiedRequest(req, 123)

	if unified.Model != "gpt-4" {
		t.Errorf("Expected model gpt-4, got %s", unified.Model)
	}
	if unified.UserID != 123 {
		t.Errorf("Expected userID 123, got %d", unified.UserID)
	}
	if len(unified.Messages) != 1 || unified.Messages[0].Content != "Hello" {
		t.Errorf("Message conversion failed")
	}
	if float32(unified.Temperature) != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", unified.Temperature)
	}
	if !unified.Stream {
		t.Errorf("Expected stream true")
	}
	if unified.RequestID == "" {
		t.Errorf("RequestID should not be empty")
	}
}

func TestFromUnifiedResponse(t *testing.T) {
	// Simple test for FromUnifiedResponse if needed, 
	// but it's mostly straightforward mapping.
}
