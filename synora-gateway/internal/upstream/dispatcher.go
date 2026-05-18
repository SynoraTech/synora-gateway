package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/synora/synora-gateway/internal/adapter"
	"github.com/synora/synora-gateway/internal/adapter/factory"
	"github.com/synora/synora-gateway/internal/router"
)

// Dispatcher handles the execution of requests to upstream providers
type Dispatcher struct {
	defaultClient *http.Client
	clients       sync.Map // map[string]*http.Client
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		defaultClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (d *Dispatcher) getClient(proxyURL *string) *http.Client {
	if proxyURL == nil || *proxyURL == "" {
		return d.defaultClient
	}
	
	pURL := *proxyURL
	if client, ok := d.clients.Load(pURL); ok {
		return client.(*http.Client)
	}

	// Create new client with proxy
	u, err := url.Parse(pURL)
	if err != nil {
		log.Printf("Warning: Invalid proxy URL %s: %v", pURL, err)
		return d.defaultClient
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(u),
	}
	newClient := &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
	}

	actual, _ := d.clients.LoadOrStore(pURL, newClient)
	return actual.(*http.Client)
}

// RequestContext holds state for a single request attempt
type RequestContext struct {
	UnifiedRequest *adapter.UnifiedRequest
	Channels       []*router.Channel
	UserTier       string
	MaxRetries     int
}

// Do executes the request with failover logic
func (d *Dispatcher) Do(ctx context.Context, rc *RequestContext, hm *router.HealthManager) (*http.Response, *router.Channel, adapter.ProviderAdapter, error) {
	var lastErr error
	triedChannels := make(map[uint32]bool)

	for i := 0; i <= rc.MaxRetries; i++ {
		// 1. Get best available channel
		ch := hm.GetBestChannel(rc.Channels, rc.UserTier)
		if ch == nil || triedChannels[ch.ID] {
			break
		}
		triedChannels[ch.ID] = true

		// 2. Get adapter for this provider
		adp, err := factory.GetAdapter(ch.Provider)
		if err != nil {
			lastErr = err
			continue
		}

		// 3. Execute request to this channel
		resp, latency, err := d.executeRequest(ctx, rc.UnifiedRequest, ch, adp)
		
		// 4. Update health based on outcome
		isSuccess := err == nil && resp != nil && resp.StatusCode < 500 && resp.StatusCode != 429
		hm.UpdateScore(ch.ID, isSuccess, latency)

		if isSuccess {
			return resp, ch, adp, nil
		}

		// 5. Handle Failover
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

		return resp, ch, adp, err
	}

	return nil, nil, nil, fmt.Errorf("all channels failed, last error: %v", lastErr)
}

func (d *Dispatcher) executeRequest(ctx context.Context, req *adapter.UnifiedRequest, ch *router.Channel, adp adapter.ProviderAdapter) (*http.Response, time.Duration, error) {
	start := time.Now()
	
	// Convert UnifiedRequest to Provider-specific body
	providerReq, err := adp.ToRequest(req)
	if err != nil {
		return nil, 0, err
	}
	body, _ := json.Marshal(providerReq)
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", ch.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	// For Anthropic, we need x-api-key and anthropic-version
	if ch.Provider == "anthropic" {
		httpReq.Header.Set("x-api-key", ch.CredentialRef)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+ch.CredentialRef)
	}

	client := d.getClient(ch.ProxyURL)
	resp, err := client.Do(httpReq)
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
