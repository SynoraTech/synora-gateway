package factory

import (
	"fmt"

	"github.com/synora/synora-gateway/internal/adapter"
	"github.com/synora/synora-gateway/internal/adapter/anthropic"
	"github.com/synora/synora-gateway/internal/adapter/gemini"
	"github.com/synora/synora-gateway/internal/adapter/openai"
)

func GetAdapter(provider string) (adapter.ProviderAdapter, error) {
	switch provider {
	case "openai":
		return openai.NewOpenAIAdapter(), nil
	case "anthropic":
		return anthropic.NewAnthropicAdapter(), nil
	case "gemini":
		return gemini.NewGeminiAdapter(), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
