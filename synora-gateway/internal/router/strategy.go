package router

import (
	"sync"
	"time"
)

type ChannelState string

const (
	StateActive      ChannelState = "active"
	StateDegraded    ChannelState = "degraded"
	StateCircuitOpen ChannelState = "circuit_open"
	StateDisabled    ChannelState = "disabled"
)

// Channel represents a physical upstream endpoint
type Channel struct {
	ID                  uint32       `json:"id"`
	Name                string       `json:"name"`
	Provider            string       `json:"provider"`
	Endpoint            string       `json:"endpoint"`
	CredentialRef       string       `json:"-"`
	Region              string       `json:"region"`
	Weight              int          `json:"weight"`
	Priority            int          `json:"priority"`
	CustomerTierAllowed string       `json:"customer_tier_allowed"`
	HealthScore         int          `json:"health_score"`
	State               ChannelState `json:"state"`
	
	mu                  sync.RWMutex
	lastFailureTime     time.Time
	failureCount        int
	
	// Metrics for sliding window (last 10 requests)
	successes []bool
	latencies []time.Duration
}

// ModelConfig maps a logical model name to available channels
type ModelConfig struct {
	ModelName string
	Channels  []*Channel
}

// HealthManager handles the dynamic health score of channels
type HealthManager struct {
	channels sync.Map // map[uint32]*Channel
}

func NewHealthManager() *HealthManager {
	return &HealthManager{}
}

// UpdateScore updates channel health based on request outcome
func (m *HealthManager) UpdateScore(channelID uint32, success bool, latency time.Duration) {
	val, ok := m.channels.Load(channelID)
	if !ok {
		return
	}
	ch := val.(*Channel)
	
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// Update sliding window
	ch.successes = append(ch.successes, success)
	ch.latencies = append(ch.latencies, latency)
	if len(ch.successes) > 10 {
		ch.successes = ch.successes[1:]
		ch.latencies = ch.latencies[1:]
	}

	// Calculate Failure Rate
	failCount := 0
	for _, s := range ch.successes {
		if !s {
			failCount++
		}
	}
	failRate := float64(failCount) / float64(len(ch.successes))

	// Calculate Max Latency (simplified for 10 samples)
	var maxLatency time.Duration
	for _, l := range ch.latencies {
		if l > maxLatency {
			maxLatency = l
		}
	}

	// HealthScore calculation (Tech Plan §3.3.2)
	score := 100.0
	score -= failRate * 40
	
	if maxLatency > 10*time.Second {
		score -= 20
	} else if maxLatency > 5*time.Second {
		score -= 10
	}

	// Recovery logic
	if success && time.Since(ch.lastFailureTime) > 5*time.Minute {
		score += 10
	}

	if score > 100 {
		score = 100
	} else if score < 0 {
		score = 0
	}

	ch.HealthScore = int(score)

	// State machine transition
	if !success {
		ch.failureCount++
		ch.lastFailureTime = time.Now()
	} else {
		ch.failureCount = 0
	}

	if ch.HealthScore < 60 || ch.failureCount >= 3 {
		ch.State = StateCircuitOpen
	} else if ch.HealthScore < 80 {
		ch.State = StateDegraded
	} else {
		ch.State = StateActive
	}
}

// GetBestChannel selects the optimal channel for a request
func (m *HealthManager) GetBestChannel(channels []*Channel, userTier string) *Channel {
	var best *Channel

	for _, ch := range channels {
		ch.mu.RLock()
		// 1. Filter by Tier (Flagship isolation)
		// Logic:
		// - If channel is for flagship but user is not, skip.
		// - If channel is for standard but user is flagship, should we prioritize flagship channels? Yes.
		
		isAllowed := false
		if ch.CustomerTierAllowed == "all" {
			isAllowed = true
		} else if ch.CustomerTierAllowed == userTier {
			isAllowed = true
		}

		if !isAllowed {
			ch.mu.RUnlock()
			continue
		}

		// 2. Filter by State
		if ch.State == StateCircuitOpen || ch.State == StateDisabled {
			ch.mu.RUnlock()
			continue
		}

		// 3. Select by Priority and Health
		// We prioritize channels that match the user tier exactly if it's not "all"
		if best == nil {
			best = ch
		} else {
			// Tier Match Priority (Flagship should use Flagship channels first)
			bestTierMatch := best.CustomerTierAllowed == userTier
			currTierMatch := ch.CustomerTierAllowed == userTier

			if currTierMatch && !bestTierMatch {
				best = ch
			} else if currTierMatch == bestTierMatch {
				if ch.Priority < best.Priority {
					best = ch
				} else if ch.Priority == best.Priority && ch.HealthScore > best.HealthScore {
					best = ch
				}
			}
		}
		ch.mu.RUnlock()
	}

	return best
}
