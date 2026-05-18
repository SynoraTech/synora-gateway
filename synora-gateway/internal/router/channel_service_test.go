package router

import (
	"testing"
)

func TestChannelService_GetChannelsForModel(t *testing.T) {
	hm := NewHealthManager()
	s := NewChannelService(hm)

	ch1 := &Channel{ID: 1, Name: "Ch1"}
	ch2 := &Channel{ID: 2, Name: "Ch2"}

	s.modelChannels = map[string][]*Channel{
		"gpt-4": {ch1, ch2},
	}

	channels := s.GetChannelsForModel("gpt-4")
	if len(channels) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(channels))
	}

	channels = s.GetChannelsForModel("unknown")
	if len(channels) != 0 {
		t.Errorf("Expected 0 channels for unknown model, got %d", len(channels))
	}
}

func TestChannelService_GetAllModelsStatus(t *testing.T) {
	hm := NewHealthManager()
	s := NewChannelService(hm)

	ch1 := &Channel{ID: 1, HealthScore: 100, State: StateActive}
	ch2 := &Channel{ID: 2, HealthScore: 50, State: StateDegraded}
	ch3 := &Channel{ID: 3, HealthScore: 0, State: StateCircuitOpen}

	s.modelChannels = map[string][]*Channel{
		"model1": {ch1},      // green (avg 100)
		"model2": {ch2},      // yellow (avg 50)
		"model3": {ch3},      // red (activeCount 0)
	}

	statuses := s.GetAllModelsStatus()
	if len(statuses) != 3 {
		t.Errorf("Expected 3 model statuses, got %d", len(statuses))
	}

	statusMap := make(map[string]ModelStatus)
	for _, st := range statuses {
		statusMap[st.ModelName] = st
	}

	if statusMap["model1"].Status != "green" {
		t.Errorf("model1 expected green, got %s", statusMap["model1"].Status)
	}
	if statusMap["model2"].Status != "yellow" {
		t.Errorf("model2 expected yellow, got %s", statusMap["model2"].Status)
	}
	if statusMap["model3"].Status != "red" {
		t.Errorf("model3 expected red, got %s", statusMap["model3"].Status)
	}
}
