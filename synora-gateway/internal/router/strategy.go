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
	// Implementation of the health algorithm from tech plan §3.3.2
	// HealthScore = 100 - (Failure Rate * 40) - (Latency Penalty * 20) ...
	
	// For MVP, start with a simple success/failure counting and circuit breaking
	val, ok := m.channels.Load(channelID)
	if !ok {
		return
	}
	ch := val.(*Channel)
	
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if success {
		ch.failureCount = 0
		ch.HealthScore = 100
		ch.State = StateActive
	} else {
		ch.failureCount++
		ch.lastFailureTime = time.Now()
		
		// If 3 consecutive failures, trip the circuit
		if ch.failureCount >= 3 {
			ch.HealthScore = 0
			ch.State = StateCircuitOpen
		} else {
			ch.HealthScore -= 30
			ch.State = StateDegraded
		}
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
