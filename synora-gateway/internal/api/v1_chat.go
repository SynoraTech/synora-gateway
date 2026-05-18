package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/synora/synora-gateway/internal/adapter/anthropic"
	"github.com/synora/synora-gateway/internal/adapter/gemini"
	adapterOpenAI "github.com/synora/synora-gateway/internal/adapter/openai"
	"github.com/synora/synora-gateway/internal/audit"
	"github.com/synora/synora-gateway/internal/billing"
	"github.com/synora/synora-gateway/internal/risk"
	"github.com/synora/synora-gateway/internal/router"
	"github.com/synora/synora-gateway/internal/upstream"
)

type ChatHandler struct {
	dispatcher *upstream.Dispatcher
	forwarder  *upstream.StreamForwarder
	hm         *router.HealthManager
	cs         *router.ChannelService
	risk       *risk.RiskEngine
	logger     *audit.LogDispatcher
	wallet     *billing.WalletService
	tokenizer  *billing.Tokenizer
}

func NewChatHandler(hm *router.HealthManager, cs *router.ChannelService, risk *risk.RiskEngine, logger *audit.LogDispatcher) *ChatHandler {
	return &ChatHandler{
		dispatcher: upstream.NewDispatcher(),
		forwarder:  upstream.NewStreamForwarder(),
		hm:         hm,
		cs:         cs,
		risk:       risk,
		logger:     logger,
		wallet:     billing.NewWalletService(),
		tokenizer:  billing.NewTokenizer(),
	}
}

func (h *ChatHandler) deductAndRecord(ctx context.Context, userID int, requestID, model, provider string, inTokens, outTokens int, logEntry *audit.CallLog) {
	price, err := h.wallet.GetModelPrice(ctx, model, provider)
	if err != nil {
		logEntry.ErrorMessage = "pricing error: " + err.Error()
		return
	}

	cost := (float64(inTokens) * price.InputPrice1k / 1000.0) + (float64(outTokens) * price.OutputPrice1k / 1000.0)
	// assuming the price is in USD, log Entry Cost is USD
	// for MVP we consider revenue = cost * 1.5, margin = cost * 0.5 roughly
	logEntry.CostUSD = cost
	logEntry.RevenueCNY = cost * 7.2 * 1.5
	logEntry.MarginCNY = cost * 7.2 * 0.5

	if cost > 0 {
		err = h.wallet.DeductBalance(ctx, userID, cost*7.2*1.5, requestID) // Deducting Revenue CNY
		if err != nil {
			logEntry.ErrorMessage = "deduct error: " + err.Error()
		}

		if h.risk != nil {
			h.risk.RecordMetric(ctx, userID, "daily_spend_cny", cost*7.2*1.5)
			h.risk.RecordMetric(ctx, userID, "spend_per_minute", cost*7.2*1.5)
		}
	}
}

// AnthropicMessages handles Anthropic native messages requests
func (h *ChatHandler) AnthropicMessages(c *gin.Context) {
	var req anthropic.MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	userID := c.GetInt("user_id")
	userTier := c.GetString("user_tier")
	if userTier == "" {
		userTier = "standard"
	}

	// Balance Check
	balance, err := h.wallet.CheckBalance(c.Request.Context(), userID)
	if err != nil || balance <= 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "Insufficient balance"})
		return
	}

	start := time.Now()

	// 1. Convert to UnifiedRequest
	unifiedReq := anthropic.ToUnifiedRequest(req, userID)
	inputTokens, _ := h.tokenizer.CountUnifiedMessagesTokens(unifiedReq.Messages, req.Model)

	// 2. Resolve Dynamic Channels
	channels := h.cs.GetChannelsForModel(req.Model)
	if len(channels) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not supported or no channels available"})
		return
	}

	rc := &upstream.RequestContext{
		UnifiedRequest: unifiedReq,
		Channels:       channels,
		UserTier:       userTier,
		MaxRetries:     2,
	}

	resp, ch, adp, err := h.dispatcher.Do(c.Request.Context(), rc, h.hm)

	logEntry := &audit.CallLog{
		RequestID:    unifiedReq.RequestID,
		UserID:       uint64(userID),
		ModelLogical: req.Model,
		TSStart:      start,
		UserAgent:    c.Request.UserAgent(),
		ClientIP:     c.ClientIP(),
		InputTokens:  uint32(inputTokens),
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

	if req.Stream {
		streamResp, err := h.forwarder.Forward(c, resp, adp)
		logEntry.TSEnd = time.Now()
		if err != nil {
			logEntry.ErrorMessage = "stream broken: " + err.Error()
		}
		
		outputTokens, _ := h.tokenizer.CountTokens(streamResp.GeneratedText, req.Model)
		logEntry.OutputTokens = uint32(outputTokens)

		h.deductAndRecord(c.Request.Context(), userID, unifiedReq.RequestID, req.Model, ch.Provider, inputTokens, outputTokens, logEntry)
		h.logger.Log(logEntry)
	} else {
		bodyBytes := h.forwarder.ProxyResponse(c, resp)
		logEntry.TSEnd = time.Now()

		outputTokens := 0
		if unifiedRes, err := adp.FromResponse(bodyBytes); err == nil {
			outputTokens = unifiedRes.Usage.CompletionTokens
			if inputTokens == 0 {
				inputTokens = unifiedRes.Usage.PromptTokens
				logEntry.InputTokens = uint32(inputTokens)
			}
		}

		logEntry.OutputTokens = uint32(outputTokens)
		h.deductAndRecord(c.Request.Context(), userID, unifiedReq.RequestID, req.Model, ch.Provider, inputTokens, outputTokens, logEntry)
		h.logger.Log(logEntry)
	}
}

// GeminiGenerateContent handles Google Gemini native generateContent requests
func (h *ChatHandler) GeminiGenerateContent(c *gin.Context) {
	model := c.Param("model")
	action := c.Param("action") // generateContent or streamGenerateContent
	
	var req gemini.GenerateContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	userID := c.GetInt("user_id")
	userTier := c.GetString("user_tier")
	if userTier == "" {
		userTier = "standard"
	}

	// Balance Check
	balance, err := h.wallet.CheckBalance(c.Request.Context(), userID)
	if err != nil || balance <= 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "Insufficient balance"})
		return
	}

	start := time.Now()
	isStream := action == "streamGenerateContent"

	// 1. Convert to UnifiedRequest
	unifiedReq := gemini.ToUnifiedRequest(model, req, userID)
	unifiedReq.Stream = isStream
	inputTokens, _ := h.tokenizer.CountUnifiedMessagesTokens(unifiedReq.Messages, model)

	// 2. Resolve Dynamic Channels
	channels := h.cs.GetChannelsForModel(model)
	if len(channels) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not supported or no channels available"})
		return
	}

	rc := &upstream.RequestContext{
		UnifiedRequest: unifiedReq,
		Channels:       channels,
		UserTier:       userTier,
		MaxRetries:     2,
	}

	resp, ch, adp, err := h.dispatcher.Do(c.Request.Context(), rc, h.hm)

	logEntry := &audit.CallLog{
		RequestID:    unifiedReq.RequestID,
		UserID:       uint64(userID),
		ModelLogical: model,
		TSStart:      start,
		UserAgent:    c.Request.UserAgent(),
		ClientIP:     c.ClientIP(),
		InputTokens:  uint32(inputTokens),
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

	if isStream {
		streamResp, err := h.forwarder.Forward(c, resp, adp)
		logEntry.TSEnd = time.Now()
		if err != nil {
			logEntry.ErrorMessage = "stream broken: " + err.Error()
		}
		
		outputTokens, _ := h.tokenizer.CountTokens(streamResp.GeneratedText, model)
		logEntry.OutputTokens = uint32(outputTokens)

		h.deductAndRecord(c.Request.Context(), userID, unifiedReq.RequestID, model, ch.Provider, inputTokens, outputTokens, logEntry)
		h.logger.Log(logEntry)
	} else {
		bodyBytes := h.forwarder.ProxyResponse(c, resp)
		logEntry.TSEnd = time.Now()

		outputTokens := 0
		if unifiedRes, err := adp.FromResponse(bodyBytes); err == nil {
			outputTokens = unifiedRes.Usage.CompletionTokens
			if inputTokens == 0 {
				inputTokens = unifiedRes.Usage.PromptTokens
				logEntry.InputTokens = uint32(inputTokens)
			}
		}

		logEntry.OutputTokens = uint32(outputTokens)
		h.deductAndRecord(c.Request.Context(), userID, unifiedReq.RequestID, model, ch.Provider, inputTokens, outputTokens, logEntry)
		h.logger.Log(logEntry)
	}
}

// ChatCompletions handles OpenAI-compatible chat completion requests
func (h *ChatHandler) ChatCompletions(c *gin.Context) {
	var req openai.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	userID := c.GetInt("user_id")
	userTier := c.GetString("user_tier")
	if userTier == "" {
		userTier = "standard"
	}

	// Balance Check
	balance, err := h.wallet.CheckBalance(c.Request.Context(), userID)
	if err != nil || balance <= 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "Insufficient balance"})
		return
	}

	start := time.Now()

	// 1. Convert to UnifiedRequest
	unifiedReq := adapterOpenAI.ToUnifiedRequest(req, userID)
	inputTokens, _ := h.tokenizer.CountUnifiedMessagesTokens(unifiedReq.Messages, req.Model)

	// 2. Resolve Dynamic Channels
	channels := h.cs.GetChannelsForModel(req.Model)
	if len(channels) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not supported or no channels available"})
		return
	}

	// 3. Dispatch with Failover
	rc := &upstream.RequestContext{
		UnifiedRequest: unifiedReq,
		Channels:       channels,
		UserTier:       userTier,
		MaxRetries:     2,
	}

	resp, ch, adp, err := h.dispatcher.Do(c.Request.Context(), rc, h.hm)

	// Create Audit Log entry
	logEntry := &audit.CallLog{
		RequestID:    unifiedReq.RequestID,
		UserID:       uint64(userID),
		ModelLogical: req.Model,
		TSStart:      start,
		UserAgent:    c.Request.UserAgent(),
		ClientIP:     c.ClientIP(),
		InputTokens:  uint32(inputTokens),
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
		streamResp, err := h.forwarder.Forward(c, resp, adp)
		logEntry.TSEnd = time.Now()
		if err != nil {
			logEntry.ErrorMessage = "stream broken: " + err.Error()
		}

		outputTokens, _ := h.tokenizer.CountTokens(streamResp.GeneratedText, req.Model)
		logEntry.OutputTokens = uint32(outputTokens)

		h.deductAndRecord(c.Request.Context(), userID, unifiedReq.RequestID, req.Model, ch.Provider, inputTokens, outputTokens, logEntry)
		h.logger.Log(logEntry)
	} else {
		bodyBytes := h.forwarder.ProxyResponse(c, resp)
		logEntry.TSEnd = time.Now()

		outputTokens := 0
		if unifiedRes, err := adp.FromResponse(bodyBytes); err == nil {
			outputTokens = unifiedRes.Usage.CompletionTokens
			if inputTokens == 0 {
				inputTokens = unifiedRes.Usage.PromptTokens
				logEntry.InputTokens = uint32(inputTokens)
			}
		}

		logEntry.OutputTokens = uint32(outputTokens)
		h.deductAndRecord(c.Request.Context(), userID, unifiedReq.RequestID, req.Model, ch.Provider, inputTokens, outputTokens, logEntry)
		h.logger.Log(logEntry)
	}
}
