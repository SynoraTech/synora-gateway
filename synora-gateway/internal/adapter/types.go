package adapter

import "time"

// Message defines a universal chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// UnifiedRequest is the internal representation of an AI request
type UnifiedRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	// Additional common fields...

	// Internal metadata
	RequestID string    `json:"-"`
	UserID    int       `json:"-"`
	StartTime time.Time `json:"-"`
}

// ProviderAdapter defines the interface for protocol-specific conversions
type ProviderAdapter interface {
	// Request conversion
	ToRequest(req *UnifiedRequest) (interface{}, error)
	
	// Response conversion
	FromResponse(body []byte) (*UnifiedResponse, error)
	
	// Stream chunk conversion (逐 chunk 转换)
	ConvertStreamChunk(line []byte) ([]byte, error)
	
	// Get completion signal for SSE
	IsStreamEnd(line []byte) bool
}

// Usage stats for tokens
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// UnifiedResponse is the internal representation of an AI response
type UnifiedResponse struct {
	ID      string    `json:"id"`
	Model   string    `json:"model"`
	Choices []Choice  `json:"choices"`
	Usage   Usage     `json:"usage"`
	Error   *AppError `json:"error,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// AppError is a standardized internal error
type AppError struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
}
