package anthropic

import (
	"testing"

	"github.com/synora/synora-gateway/internal/adapter"
)

func TestToUnifiedRequest(t *testing.T) {
	req := MessageRequest{
		Model: "claude-3-opus-20240229",
		Messages: []ContentBlock{
			{Role: "user", Content: "Hello Claude"},
		},
		System:    "You are a helpful assistant",
		MaxTokens: 1024,
		Stream:    true,
	}

	unified := ToUnifiedRequest(req, 123)

	if unified.Model != "claude-3-opus-20240229" {
		t.Errorf("Expected model claude-3-opus-20240229, got %s", unified.Model)
	}
	if unified.UserID != 123 {
		t.Errorf("Expected userID 123, got %d", unified.UserID)
	}
	if len(unified.Messages) != 2 {
		t.Errorf("Expected 2 messages (system + user), got %d", len(unified.Messages))
	}
	if unified.Messages[0].Role != "system" || unified.Messages[0].Content != "You are a helpful assistant" {
		t.Errorf("System message conversion failed")
	}
	if unified.Messages[1].Role != "user" || unified.Messages[1].Content != "Hello Claude" {
		t.Errorf("User message conversion failed")
	}
	if unified.MaxTokens != 1024 {
		t.Errorf("Expected MaxTokens 1024, got %d", unified.MaxTokens)
	}
	if !unified.Stream {
		t.Errorf("Expected stream true")
	}
}

func TestUnifiedToAnthropic(t *testing.T) {
	unified := &adapter.UnifiedRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []adapter.Message{
			{Role: "system", Content: "System prompt"},
			{Role: "user", Content: "User query"},
		},
		MaxTokens: 512,
		Stream:    false,
	}

	req := UnifiedToAnthropic(unified)

	if req.Model != "claude-3-sonnet-20240229" {
		t.Errorf("Expected model claude-3-sonnet-20240229, got %s", req.Model)
	}
	if req.System != "System prompt" {
		t.Errorf("Expected system prompt, got %s", req.System)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "User query" {
		t.Errorf("Messages conversion failed")
	}
	if req.MaxTokens != 512 {
		t.Errorf("Expected max_tokens 512, got %d", req.MaxTokens)
	}
	if req.Stream {
		t.Errorf("Expected stream false")
	}
}

func TestAnthropicAdapter_FromResponse(t *testing.T) {
	a := NewAnthropicAdapter()
	respJSON := `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"model": "claude-3-opus",
		"content": [
			{"type": "text", "content": "Hello! How can I help you today?"}
		],
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 10,
			"output_tokens": 20
		}
	}`

	res, err := a.FromResponse([]byte(respJSON))
	if err != nil {
		t.Fatalf("FromResponse failed: %v", err)
	}

	if res.ID != "msg_123" {
		t.Errorf("Expected ID msg_123, got %s", res.ID)
	}
	if res.Choices[0].Message.Content != "Hello! How can I help you today?" {
		t.Errorf("Content mismatch")
	}
	if res.Usage.PromptTokens != 10 || res.Usage.CompletionTokens != 20 {
		t.Errorf("Usage mismatch")
	}
}
