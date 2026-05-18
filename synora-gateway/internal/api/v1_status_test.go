package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/synora/synora-gateway/internal/router"
)

func TestStatusHandler_GetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	hm := router.NewHealthManager()
	cs := router.NewChannelService(hm)
	
	// Manually inject some models status if we can
	// Since cs is in another package, we can't set modelChannels directly
	// unless we use a trick or refactor.
	// But wait, router.ChannelService is in 'router' package, api is in 'api' package.
	
	h := NewStatusHandler(cs)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	h.GetStatus(c)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	
	if resp["status"] != "unknown" { // because models are empty
		t.Errorf("Expected status unknown, got %s", resp["status"])
	}
}
