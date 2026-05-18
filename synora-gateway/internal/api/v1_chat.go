package api

import (
	"net/http"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	adapterOpenAI "github.com/synora/synora-gateway/internal/adapter/openai"
	"github.com/synora/synora-gateway/internal/audit"
	"github.com/synora/synora-gateway/internal/router"
	"github.com/synora/synora-gateway/internal/upstream"
)

type ChatHandler struct {
	dispatcher *upstream.Dispatcher
	forwarder  *upstream.StreamForwarder
	hm         *router.HealthManager
	logger     *audit.LogDispatcher
}

func NewChatHandler(hm *router.HealthManager, logger *audit.LogDispatcher) *ChatHandler {
	return &ChatHandler{
		dispatcher: upstream.NewDispatcher(),
		forwarder:  upstream.NewStreamForwarder(),
		hm:         hm,
		logger:     logger,
	}
}

// ChatCompletions handles OpenAI-compatible chat completion requests
func (h *ChatHandler) ChatCompletions(c *gin.Context) {
	// ... (Existing implementation)
	var req openai.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	userID := c.GetInt("user_id")
	start := time.Now()

	// 1. Convert to UnifiedRequest
	unifiedReq := adapterOpenAI.ToUnifiedRequest(req, userID)

	// 2. Mock Channels for MVP
	channels := []*router.Channel{
		{
			ID:            1,
			Name:          "OpenAI-Primary",
			Provider:      "openai",
			Endpoint:      "https://api.openai.com/v1/chat/completions",
			CredentialRef: "sk-...",
			Priority:      1,
			HealthScore:   100,
			State:         router.StateActive,
		},
	}

	// 3. Dispatch with Failover
	rc := &upstream.RequestContext{
		UnifiedRequest: unifiedReq,
		Channels:       channels,
		UserTier:       "standard",
		MaxRetries:     2,
	}

	resp, ch, err := h.dispatcher.Do(c.Request.Context(), rc, h.hm)
	
	// Create Audit Log entry
	logEntry := &audit.CallLog{
		RequestID:    unifiedReq.RequestID,
		UserID:       uint64(userID),
		ModelLogical: req.Model,
		TSStart:      start,
		UserAgent:    c.Request.UserAgent(),
		ClientIP:     c.ClientIP(),
	}
	if ch != nil {
		logEntry.ChannelID = ch.ID
		logEntry.Provider = ch.Provider
	}

	if err != nil {
		logEntry.TSEnd = time.Now()
		logEntry.StatusCode = http.StatusServiceUnavailable
		logEntry.ErrorMessage = err.Error()
		h.logger.Log(logEntry)

		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	logEntry.StatusCode = uint16(resp.StatusCode)

	// 4. Handle Response
	if req.Stream {
		_, err := h.forwarder.Forward(c, resp)
		logEntry.TSEnd = time.Now()
		if err != nil {
			logEntry.ErrorMessage = "stream broken: " + err.Error()
		}
		h.logger.Log(logEntry)
	} else {
		h.forwarder.ProxyResponse(c, resp)
		logEntry.TSEnd = time.Now()
		h.logger.Log(logEntry)
	}
}
