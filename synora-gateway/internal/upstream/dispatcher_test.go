package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/synora/synora-gateway/internal/adapter"
	"github.com/synora/synora-gateway/internal/router"
)

func TestDispatcher_Do_Failover(t *testing.T) {
	// 1. Setup mock server that fails first then succeeds
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"content": "Success"}}]}`))
	}))
	defer server.Close()

	d := NewDispatcher()
	hm := router.NewHealthManager()

	ch1 := &router.Channel{ID: 1, Provider: "openai", Endpoint: server.URL, Priority: 1, HealthScore: 100, State: router.StateActive, CustomerTierAllowed: "all"}
	ch2 := &router.Channel{ID: 2, Provider: "openai", Endpoint: server.URL, Priority: 1, HealthScore: 100, State: router.StateActive, CustomerTierAllowed: "all"}
	
	hm.AddChannel(ch1)
	hm.AddChannel(ch2)

	rc := &RequestContext{
		UnifiedRequest: &adapter.UnifiedRequest{Model: "gpt-3.5-turbo"},
		Channels:       []*router.Channel{ch1, ch2},
		UserTier:       "standard",
		MaxRetries:     1,
	}

	resp, ch, _, err := d.Do(context.Background(), rc, hm)

	if err != nil {
		t.Fatalf("Expected success after failover, got error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
	if ch.ID != 2 {
		t.Errorf("Expected failover to channel 2, got %d", ch.ID)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %d", resp.StatusCode)
	}
}

func TestShouldFailover(t *testing.T) {
	tests := []struct {
		statusCode int
		err        error
		want       bool
	}{
		{http.StatusOK, nil, false},
		{http.StatusBadRequest, nil, false},
		{http.StatusInternalServerError, nil, true},
		{http.StatusTooManyRequests, nil, true},
		{0, context.DeadlineExceeded, true},
	}

	for _, tt := range tests {
		var resp *http.Response
		if tt.statusCode != 0 {
			resp = &http.Response{StatusCode: tt.statusCode}
		}
		if got := shouldFailover(tt.err, resp); got != tt.want {
			t.Errorf("shouldFailover(%d, %v) = %v, want %v", tt.statusCode, tt.err, got, tt.want)
		}
	}
}
