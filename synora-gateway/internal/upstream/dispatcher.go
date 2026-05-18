package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/synora/synora-gateway/internal/adapter"
	"github.com/synora/synora-gateway/internal/router"
)

// Dispatcher handles the execution of requests to upstream providers
type Dispatcher struct {
	httpClient *http.Client
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// RequestContext holds state for a single request attempt
type RequestContext struct {
	UnifiedRequest *adapter.UnifiedRequest
	Channels       []*router.Channel
	UserTier       string
	MaxRetries     int
}

// Do executes the request with failover logic
func (d *Dispatcher) Do(ctx context.Context, rc *RequestContext, hm *router.HealthManager) (*http.Response, *router.Channel, error) {
	var lastErr error
	triedChannels := make(map[uint32]bool)

	for i := 0; i <= rc.MaxRetries; i++ {
		// 1. Get best available channel
		ch := hm.GetBestChannel(rc.Channels, rc.UserTier)
		if ch == nil || triedChannels[ch.ID] {
			break
		}
		triedChannels[ch.ID] = true

		// 2. Execute request to this channel
		resp, latency, err := d.executeRequest(ctx, rc.UnifiedRequest, ch)
		
		// 3. Update health based on outcome
		isSuccess := err == nil && resp.StatusCode < 500 && resp.StatusCode != 429
		hm.UpdateScore(ch.ID, isSuccess, latency)

		if isSuccess {
			return resp, ch, nil
		}

		// 4. Handle Failover
		if i < rc.MaxRetries && shouldFailover(err, resp) {
			if resp != nil {
				resp.Body.Close()
			}
			lastErr = err
			if err == nil {
				lastErr = fmt.Errorf("upstream status: %d", resp.StatusCode)
			}
			continue
		}

		return resp, ch, err
	}

	return nil, nil, fmt.Errorf("all channels failed, last error: %v", lastErr)
}

func (d *Dispatcher) executeRequest(ctx context.Context, req *adapter.UnifiedRequest, ch *router.Channel) (*http.Response, time.Duration, error) {
	start := time.Now()
	
	// Convert UnifiedRequest to Provider-specific body (MVP: assume OpenAI)
	body, _ := json.Marshal(req) // Simplified for MVP
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", ch.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}

	// Set headers (Credentials)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+ch.CredentialRef) // In real, decrypt this

	resp, err := d.httpClient.Do(httpReq)
	latency := time.Since(start)

	return resp, latency, err
}

func shouldFailover(err error, resp *http.Response) bool {
	if err != nil {
		return true // Network errors, timeouts
	}
	if resp == nil {
		return true
	}
	// Failover on 5xx or 429 (Rate Limit)
	return resp.StatusCode >= 500 || resp.StatusCode == 429
}
