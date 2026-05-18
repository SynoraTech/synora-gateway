package router

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/synora/synora-gateway/internal/storage/pg"
)

// ChannelService handles loading and caching channel configurations
type ChannelService struct {
	hm            *HealthManager
	modelChannels map[string][]*Channel
	mu            sync.RWMutex
}

func NewChannelService(hm *HealthManager) *ChannelService {
	return &ChannelService{
		hm:            hm,
		modelChannels: make(map[string][]*Channel),
	}
}

// LoadAllChannels loads channels from DB and updates the HealthManager and local cache
func (s *ChannelService) LoadAllChannels(ctx context.Context) error {
	db := pg.GetDB()

	// 1. Load all channels
	rows, err := db.Query(ctx, `
		SELECT id, name, provider, endpoint, credential_ref, region, weight, priority, customer_tier_allowed, health_score, state 
		FROM channels
	`)
	if err != nil {
		return fmt.Errorf("failed to query channels: %v", err)
	}
	defer rows.Close()

	allChannels := make(map[uint32]*Channel)
	for rows.Next() {
		ch := &Channel{}
		err := rows.Scan(
			&ch.ID, &ch.Name, &ch.Provider, &ch.Endpoint, &ch.CredentialRef, 
			&ch.Region, &ch.Weight, &ch.Priority, &ch.CustomerTierAllowed, 
			&ch.HealthScore, &ch.State,
		)
		if err != nil {
			return fmt.Errorf("failed to scan channel: %v", err)
		}
		allChannels[ch.ID] = ch
		
		// Update HealthManager map if not already there
		if _, ok := s.hm.channels.Load(ch.ID); !ok {
			s.hm.channels.Store(ch.ID, ch)
		}
	}

	// 2. Load model-channel mappings
	mappingRows, err := db.Query(ctx, "SELECT model_name, channel_id FROM model_channels")
	if err != nil {
		return fmt.Errorf("failed to query model mappings: %v", err)
	}
	defer mappingRows.Close()

	newModelChannels := make(map[string][]*Channel)
	for mappingRows.Next() {
		var modelName string
		var channelID uint32
		if err := mappingRows.Scan(&modelName, &channelID); err != nil {
			return err
		}
		if ch, ok := allChannels[channelID]; ok {
			newModelChannels[modelName] = append(newModelChannels[modelName], ch)
		}
	}

	s.mu.Lock()
	s.modelChannels = newModelChannels
	s.mu.Unlock()

	log.Printf("Loaded %d channels and %d model mappings", len(allChannels), len(newModelChannels))
	return nil
}

// GetChannelsForModel returns the list of physical channels for a logical model
func (s *ChannelService) GetChannelsForModel(modelName string) []*Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelChannels[modelName]
}
