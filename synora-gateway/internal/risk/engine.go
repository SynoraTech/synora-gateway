package risk

import (
	"context"
	"fmt"
	"os"

	"github.com/anknown/ahocorasick"
	"gopkg.in/yaml.v3"
)

// RiskRule defines a declarative risk policy
type RiskRule struct {
	ID        string  `yaml:"id"`
	Scope     string  `yaml:"scope"`
	Metric    string  `yaml:"metric"`
	Threshold float64 `yaml:"threshold"`
	Action    string  `yaml:"action"`
}

type Config struct {
	Rules []RiskRule `yaml:"rules"`
}

// RiskEngine handles content moderation and spend control
type RiskEngine struct {
	ac      *ahocorasick.Machine
	config  *Config
}

func NewRiskEngine(rulesPath string, keywords []string) (*RiskEngine, error) {
	// 1. Build AC machine for fast keyword matching
	machine := new(ahocorasick.Machine)
	dict := make([][]rune, len(keywords))
	for i, k := range keywords {
		dict[i] = []rune(k)
	}
	if err := machine.Build(dict); err != nil {
		return nil, fmt.Errorf("failed to build AC machine: %v", err)
	}

	// 2. Load rules from YAML
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read risk rules: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal risk rules: %v", err)
	}

	return &RiskEngine{
		ac:     machine,
		config: &cfg,
	}, nil
}

// ModerateContent checks for blacklisted keywords
func (e *RiskEngine) ModerateContent(text string) (bool, string) {
	hits := e.ac.MultiPatternSearch([]rune(text), false)
	if len(hits) > 0 {
		// Return true (hit) and the first matching keyword (for audit)
		return true, string(hits[0].Word)
	}
	return false, ""
}

// EvaluateRules checks if any business rules are violated
func (e *RiskEngine) EvaluateRules(ctx context.Context, userID int, metrics map[string]float64) (bool, string) {
	for _, rule := range e.config.Rules {
		if val, ok := metrics[rule.Metric]; ok {
			if val >= rule.Threshold {
				return true, fmt.Sprintf("Rule %s triggered: %.2f >= %.2f", rule.ID, val, rule.Threshold)
			}
		}
	}
	return false, ""
}
