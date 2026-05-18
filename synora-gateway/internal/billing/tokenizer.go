package billing

import (
	"fmt"
	"sync"

	"github.com/pkoukk/tiktoken-go"
	"github.com/synora/synora-gateway/internal/adapter"
)

// Tokenizer handles token counting for different models
type Tokenizer struct {
	mu        sync.Mutex
	encodings map[string]*tiktoken.Tiktoken
}

func NewTokenizer() *Tokenizer {
	return &Tokenizer{
		encodings: make(map[string]*tiktoken.Tiktoken),
	}
}

// CountTokens estimates the number of tokens in a string for a given model
func (t *Tokenizer) CountTokens(text string, model string) (int, error) {
	if text == "" {
		return 0, nil
	}
	// Map common models to their encodings
	encoding := "cl100k_base" // Default for GPT-4, GPT-3.5-turbo
	if model == "gpt-4o" || model == "gpt-4o-mini" {
		encoding = "o200k_base"
	}

	t.mu.Lock()
	tke, ok := t.encodings[encoding]
	if !ok {
		var err error
		tke, err = tiktoken.GetEncoding(encoding)
		if err == nil {
			t.encodings[encoding] = tke
		}
	}
	t.mu.Unlock()
	
	if tke == nil {
		// If tke is still nil, it means GetEncoding failed
		// Re-fetch without cache to get the actual error if it failed
		var err error
		tke, err = tiktoken.GetEncoding(encoding)
		if err != nil {
			return 0, fmt.Errorf("failed to get encoding: %v", err)
		}
	}

	token := tke.Encode(text, nil, nil)
	return len(token), nil
}

// CountMessagesTokens counts tokens for OpenAI message list
func (t *Tokenizer) CountMessagesTokens(messages []interface{}, model string) (int, error) {
	// Simplified implementation for MVP
	total := 0
	for _, m := range messages {
		if msg, ok := m.(map[string]string); ok {
			c, err := t.CountTokens(msg["content"], model)
			if err != nil {
				return 0, err
			}
			total += c + 4 // Add constant overhead per message
		}
	}
	return total + 3, nil // Add assistant response overhead
}

// CountUnifiedMessagesTokens counts tokens for UnifiedRequest messages
func (t *Tokenizer) CountUnifiedMessagesTokens(messages []adapter.Message, model string) (int, error) {
	total := 0
	for _, m := range messages {
		c, err := t.CountTokens(m.Content, model)
		if err != nil {
			return 0, err
		}
		total += c + 4 // Add constant overhead per message
	}
	return total + 3, nil
}
