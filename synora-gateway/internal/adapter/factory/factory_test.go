package factory

import (
	"testing"
)

func TestGetAdapter(t *testing.T) {
	tests := []struct {
		provider string
		wantErr  bool
	}{
		{"openai", false},
		{"anthropic", false},
		{"gemini", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			adp, err := GetAdapter(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && adp == nil {
				t.Errorf("GetAdapter() returned nil adapter for %s", tt.provider)
			}
		})
	}
}
