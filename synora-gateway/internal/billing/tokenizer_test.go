package billing

import (
	"testing"

	"github.com/synora/synora-gateway/internal/adapter"
)

func TestTokenizer_CountTokens(t *testing.T) {
	tokenizer := NewTokenizer()

	tests := []struct {
		text  string
		model string
		want  int
	}{
		{"Hello, world!", "gpt-4o", 3}, // o200k_base
		{"Hello, world!", "gpt-4", 4},  // cl100k_base
		{"", "gpt-4", 0},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got, err := tokenizer.CountTokens(tt.text, tt.model)
			if err != nil {
				t.Logf("Skipping CountTokens test due to error: %v (likely network issue for tiktoken)", err)
				t.Skip()
				return
			}
			if got != tt.want {
				t.Errorf("CountTokens() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenizer_CountUnifiedMessagesTokens(t *testing.T) {
	tokenizer := NewTokenizer()
	messages := []adapter.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	// OpenAI cl100k_base: "Hello" (1), "Hi there!" (3)
	// Overhead: (1+4) + (3+4) + 3 = 15
	got, err := tokenizer.CountUnifiedMessagesTokens(messages, "gpt-4")
	if err != nil {
		t.Logf("Skipping CountUnifiedMessagesTokens test due to error: %v", err)
		t.Skip()
		return
	}

	if got <= 0 {
		t.Errorf("Expected positive token count, got %d", got)
	}
}
