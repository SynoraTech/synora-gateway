package audit

import (
	"context"
	"log"
	"time"

	"github.com/synora/synora-gateway/internal/storage/clickhouse"
)

// CallLog represents a single request log entry
type CallLog struct {
	RequestID     string
	UserID        uint64
	APIKeyID      uint64
	CustomerTier  string
	ModelLogical  string
	ChannelID     uint32
	Provider      string
	Region        string
	TSStart       time.Time
	TSFirstToken  time.Time
	TSEnd         time.Time
	StatusCode    uint16
	IsFailover    uint8
	FailoverChain []uint32
	InputTokens   uint32
	OutputTokens  uint32
	CostUSD       float64
	RevenueCNY    float64
	MarginCNY     float64
	ErrorType     string
	ErrorMessage  string
	ModerationHit uint8
	ClientIP      string
	UserAgent     string
}

// LogDispatcher handles asynchronous log writing
type LogDispatcher struct {
	logChan chan *CallLog
}

func NewLogDispatcher(bufferSize int) *LogDispatcher {
	d := &LogDispatcher{
		logChan: make(chan *CallLog, bufferSize),
	}
	go d.startWorker()
	return d
}

func (d *LogDispatcher) Log(entry *CallLog) {
	select {
	case d.logChan <- entry:
	default:
		log.Println("Audit log channel full, dropping log entry")
	}
}

func (d *LogDispatcher) startWorker() {
	conn := clickhouse.GetConn()
	
	for entry := range d.logChan {
		ctx := context.Background()
		err := conn.Exec(ctx, `
			INSERT INTO call_logs (
				request_id, user_id, api_key_id, customer_tier, model_logical,
				channel_id, provider, region, ts_start, ts_first_token, ts_end,
				status_code, is_failover, failover_chain, input_tokens, output_tokens,
				cost_usd, revenue_cny, margin_cny, error_type, error_message,
				moderation_hit, client_ip, user_agent
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			entry.RequestID, entry.UserID, entry.APIKeyID, entry.CustomerTier, entry.ModelLogical,
			entry.ChannelID, entry.Provider, entry.Region, entry.TSStart, entry.TSFirstToken, entry.TSEnd,
			entry.StatusCode, entry.IsFailover, entry.FailoverChain, entry.InputTokens, entry.OutputTokens,
			entry.CostUSD, entry.RevenueCNY, entry.MarginCNY, entry.ErrorType, entry.ErrorMessage,
			entry.ModerationHit, entry.ClientIP, entry.UserAgent,
		)

		if err != nil {
			log.Printf("Failed to write audit log to ClickHouse: %v", err)
		}
	}
}
