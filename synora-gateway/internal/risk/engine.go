package risk

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/anknown/ahocorasick"
	"github.com/synora/synora-gateway/internal/storage/pg"
	"github.com/synora/synora-gateway/internal/storage/redis"
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
	ac     *goahocorasick.Machine
	config *Config
}

func NewRiskEngine(rulesPath string, keywords []string) (*RiskEngine, error) {
	// ... (Existing AC and YAML loading)
	var machine *goahocorasick.Machine
	if len(keywords) > 0 {
		machine = new(goahocorasick.Machine)
		dict := make([][]rune, len(keywords))
		for i, k := range keywords {
			dict[i] = []rune(k)
		}
		if err := machine.Build(dict); err != nil {
			return nil, fmt.Errorf("failed to build AC machine: %v", err)
		}
	}

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

// RecordMetric updates real-time metrics in Redis
func (e *RiskEngine) RecordMetric(ctx context.Context, userID int, metric string, value float64) error {
	rdb := redis.GetRedis()
	now := time.Now()
	
	switch metric {
	case "daily_spend_cny":
		key := fmt.Sprintf("risk:metric:daily_spend:%d:%s", userID, now.Format("20060102"))
		return rdb.IncrByFloat(ctx, key, value).Err()
	case "spend_per_minute":
		key := fmt.Sprintf("risk:metric:min_spend:%d:%s", userID, now.Format("200601021504"))
		err := rdb.IncrByFloat(ctx, key, value).Err()
		rdb.Expire(ctx, key, 2*time.Minute)
		return err
	}
	return nil
}

// GetMetrics retrieves current metrics from Redis
func (e *RiskEngine) GetMetrics(ctx context.Context, userID int) (map[string]float64, error) {
	rdb := redis.GetRedis()
	now := time.Now()
	metrics := make(map[string]float64)

	// Daily Spend
	dailyKey := fmt.Sprintf("risk:metric:daily_spend:%d:%s", userID, now.Format("20060102"))
	val, _ := rdb.Get(ctx, dailyKey).Float64()
	metrics["daily_spend_cny"] = val

	// Per Minute Spend
	minKey := fmt.Sprintf("risk:metric:min_spend:%d:%s", userID, now.Format("200601021504"))
	val, _ = rdb.Get(ctx, minKey).Float64()
	metrics["spend_per_minute"] = val

	return metrics, nil
}

// ModerateContent checks for blacklisted keywords
func (e *RiskEngine) ModerateContent(text string) (bool, string) {
	if e.ac == nil {
		return false, ""
	}
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

// StartRiskEvaluationLoop runs a background worker to periodically check all active users
func (e *RiskEngine) StartRiskEvaluationLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.evaluateAllUsers(ctx)
		}
	}
}

func (e *RiskEngine) evaluateAllUsers(ctx context.Context) {
	db := pg.GetDB()
	// Get all users who have been active in the last hour to minimize scanning
	rows, err := db.Query(ctx, "SELECT id FROM users") // Simplified: scan all for MVP
	if err != nil {
		log.Printf("Risk Error: failed to query users: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var userID int
		if err := rows.Scan(&userID); err != nil {
			continue
		}

		metrics, err := e.GetMetrics(ctx, userID)
		if err != nil {
			continue
		}

		if triggered, msg := e.EvaluateRules(ctx, userID, metrics); triggered {
			log.Printf("RISK TRIGGERED: User %d: %s", userID, msg)
			e.takeAction(ctx, userID, msg)
		}
	}
}

func (e *RiskEngine) takeAction(ctx context.Context, userID int, msg string) {
	db := pg.GetDB()
	// Default action: pause all API keys for this user
	_, err := db.Exec(ctx, "UPDATE api_keys SET status = 'paused' WHERE user_id = $1 AND status = 'active'", userID)
	if err != nil {
		log.Printf("Risk Error: failed to take action for user %d: %v", userID, err)
		return
	}
	log.Printf("Risk Action: Paused all active keys for User %d due to: %s", userID, msg)
}

