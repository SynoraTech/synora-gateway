package billing

import (
	"fmt"

	"github.com/pkoukk/tiktoken-go"
)

// Tokenizer handles token counting for different models
type Tokenizer struct {
	// Cache for encodings if needed
}

func NewTokenizer() *Tokenizer {
	return &Tokenizer{}
}

// CountTokens estimates the number of tokens in a string for a given model
func (t *Tokenizer) CountTokens(text string, model string) (int, error) {
	// Map common models to their encodings
	encoding := "cl100k_base" // Default for GPT-4, GPT-3.5-turbo
	if model == "gpt-4o" || model == "gpt-4o-mini" {
		encoding = "o200k_base"
	}

	tke, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return 0, fmt.Errorf("failed to get encoding: %v", err)
	}

	token := tke.Encode(text, nil, nil)
	return len(token), nil
}

// CountMessagesTokens counts tokens for OpenAI message list
func (t *Tokenizer) CountMessagesTokens(messages []interface{}, model string) (int, error) {
	// Simplified implementation for MVP
	// In production, this needs to account for message structure overhead
	total := 0
	for _, m := range messages {
		if msg, ok := m.(map[string]string); ok {
			c, _ := t.CountTokens(msg["content"], model)
			total += c + 4 // Add constant overhead per message
		}
	}
	return total + 3, nil // Add assistant response overhead
}
