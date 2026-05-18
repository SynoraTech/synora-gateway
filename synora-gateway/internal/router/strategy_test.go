package router

import (
	"testing"
	"time"
)

func TestHealthManager_UpdateScore(t *testing.T) {
	hm := NewHealthManager()
	ch := &Channel{
		ID:          1,
		Name:        "TestChannel",
		HealthScore: 100,
		State:       StateActive,
	}
	hm.channels.Store(ch.ID, ch)

	// 1. Test single failure
	hm.UpdateScore(ch.ID, false, 100*time.Millisecond)
	if ch.HealthScore != 60 {
		t.Errorf("Expected health score 60, got %d", ch.HealthScore)
	}
	if ch.State != StateDegraded {
		t.Errorf("Expected state degraded, got %s", ch.State)
	}

	// 2. Test 3 consecutive failures (Circuit Break)
	hm.UpdateScore(ch.ID, false, 100*time.Millisecond)
	hm.UpdateScore(ch.ID, false, 100*time.Millisecond)
	if ch.HealthScore != 60 { // failRate is 1.0, score remains 60
		t.Errorf("Expected health score 60, got %d", ch.HealthScore)
	}
	if ch.State != StateCircuitOpen {
		t.Errorf("Expected state circuit_open, got %s", ch.State)
	}

	// 3. Test recovery on success
	hm.UpdateScore(ch.ID, true, 50*time.Millisecond)
	// failRate = 3/4 = 0.75. score = 100 - 0.75*40 = 70.
	if ch.HealthScore != 70 {
		t.Errorf("Expected health score 70, got %d", ch.HealthScore)
	}
	if ch.State != StateDegraded {
		t.Errorf("Expected state degraded, got %s", ch.State)
	}
}

func TestGetBestChannel(t *testing.T) {
	hm := NewHealthManager()
	
	ch1 := &Channel{ID: 1, Priority: 1, HealthScore: 100, State: StateActive, CustomerTierAllowed: "all"}
	ch2 := &Channel{ID: 2, Priority: 1, HealthScore: 80, State: StateActive, CustomerTierAllowed: "all"}
	ch3 := &Channel{ID: 3, Priority: 2, HealthScore: 100, State: StateActive, CustomerTierAllowed: "all"}
	ch4 := &Channel{ID: 4, Priority: 1, HealthScore: 100, State: StateActive, CustomerTierAllowed: "flagship"}

	channels := []*Channel{ch1, ch2, ch3, ch4}

	// Case 1: Standard user should get ch1 (best priority & health)
	best := hm.GetBestChannel(channels, "standard")
	if best.ID != 1 {
		t.Errorf("Standard user: expected channel 1, got %d", best.ID)
	}

	// Case 2: Flagship user should get ch4 (exclusive)
	best = hm.GetBestChannel(channels, "flagship")
	if best.ID != 4 {
		t.Errorf("Flagship user: expected channel 4, got %d", best.ID)
	}

	// Case 3: Health score drop
	ch1.mu.Lock()
	ch1.HealthScore = 50
	ch1.mu.Unlock()
	best = hm.GetBestChannel(channels, "standard")
	if best.ID != 2 {
		t.Errorf("After health drop: expected channel 2, got %d", best.ID)
	}
}
