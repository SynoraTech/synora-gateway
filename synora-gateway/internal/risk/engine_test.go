package risk

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRiskEngine_ModerateContent(t *testing.T) {
	// Setup a temporary rule file
	rulesContent := `
rules:
  - id: test_rule
    scope: user
    metric: daily_spend_cny
    threshold: 100
    action: pause_key
`
	rulesPath := "test_rules.yaml"
	os.WriteFile(rulesPath, []byte(rulesContent), 0644)
	defer os.Remove(rulesPath)

	keywords := []string{"forbidden", "illegal"}
	engine, err := NewRiskEngine(rulesPath, keywords)
	if err != nil {
		t.Fatalf("Failed to create RiskEngine: %v", err)
	}

	tests := []struct {
		text string
		want bool
	}{
		{"This is a safe text", false},
		{"This contains forbidden word", true},
		{"ILLEGAL content", false}, // AC machine is case-sensitive by default with current impl
		{"illegal content", true},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got, word := engine.ModerateContent(tt.text)
			if got != tt.want {
				t.Errorf("ModerateContent() got = %v, want %v (word: %s)", got, tt.want, word)
			}
		})
	}
}

func TestRiskEngine_EvaluateRules(t *testing.T) {
	rulesContent := `
rules:
  - id: daily_limit
    metric: daily_spend_cny
    threshold: 50
  - id: burst_limit
    metric: spend_per_minute
    threshold: 10
`
	rulesPath := "test_rules_eval.yaml"
	os.WriteFile(rulesPath, []byte(rulesContent), 0644)
	defer os.Remove(rulesPath)

	engine, err := NewRiskEngine(rulesPath, nil)
	if err != nil {
		t.Fatalf("Failed to create RiskEngine: %v", err)
	}

	tests := []struct {
		name     string
		metrics  map[string]float64
		want     bool
		wantRule string
	}{
		{"Under limits", map[string]float64{"daily_spend_cny": 10, "spend_per_minute": 1}, false, ""},
		{"Over daily", map[string]float64{"daily_spend_cny": 60, "spend_per_minute": 1}, true, "daily_limit"},
		{"Over burst", map[string]float64{"daily_spend_cny": 10, "spend_per_minute": 15}, true, "burst_limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, msg := engine.EvaluateRules(context.Background(), 1, tt.metrics)
			if got != tt.want {
				t.Errorf("EvaluateRules() got = %v, want %v", got, tt.want)
			}
			if got && tt.wantRule != "" && !strings.Contains(msg, tt.wantRule) {
				t.Errorf("EvaluateRules() message '%s' does not contain expected rule '%s'", msg, tt.wantRule)
			}
		})
	}
}
