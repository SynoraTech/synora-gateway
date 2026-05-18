package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/synora/synora-gateway/internal/router"
)

type StatusHandler struct {
	cs *router.ChannelService
}

func NewStatusHandler(cs *router.ChannelService) *StatusHandler {
	return &StatusHandler{cs: cs}
}

// GetStatus returns the public aggregated status of models
func (h *StatusHandler) GetStatus(c *gin.Context) {
	models := h.cs.GetAllModelsStatus()
	
	// If the system has models and at least one is operational, we consider it operational
	overallStatus := "operational"
	if len(models) == 0 {
		overallStatus = "unknown"
	}
	for _, m := range models {
		if m.Status == "red" {
			overallStatus = "degraded"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": overallStatus,
		"models": models,
		// In the future, read P95 and historical events from ClickHouse
	})
}
