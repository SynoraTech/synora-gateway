package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/synora/synora-gateway/internal/billing"
	"github.com/synora/synora-gateway/internal/risk"
	"github.com/synora/synora-gateway/internal/storage/pg"
	"github.com/synora/synora-gateway/internal/storage/redis"
)

const (
	AuthHeader   = "Authorization"
	BearerPrefix = "Bearer "
)

// AuthMiddleware handles API Key authentication, balance pre-check, and risk control
func AuthMiddleware(riskEngine *risk.RiskEngine) gin.HandlerFunc {
	walletSvc := billing.NewWalletService()

	return func(c *gin.Context) {
		authHeader := c.GetHeader(AuthHeader)
		if authHeader == "" || !strings.HasPrefix(authHeader, BearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}

		apiKey := strings.TrimPrefix(authHeader, BearerPrefix)
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Empty API key"})
			return
		}

		keyHash := hashAPIKey(apiKey)
		ctx := c.Request.Context()
		rdb := redis.GetRedis()
		authCacheKey := fmt.Sprintf("auth:key:%s", keyHash)
		
		userIDStr, err := rdb.Get(ctx, authCacheKey).Result()
		var userID int
		if err == nil && userIDStr != "" {
			parts := strings.Split(userIDStr, ":")
			userID, _ = strconv.Atoi(parts[0])
			if len(parts) > 1 {
				c.Set("user_tier", parts[1])
			}
		} else {
			db := pg.GetDB()
			var status string
			var tier string
			query := `
				SELECT k.user_id, k.status, u.tier 
				FROM api_keys k 
				JOIN users u ON k.user_id = u.id 
				WHERE k.key_hash = $1
			`
			err = db.QueryRow(ctx, query, keyHash).Scan(&userID, &status, &tier)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
				return
			}
			if status != "active" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API key is " + status})
				return
			}
			rdb.Set(ctx, authCacheKey, fmt.Sprintf("%d:%s", userID, tier), 300*1e9)
			c.Set("user_tier", tier)
		}

		// 1. Risk Evaluation (declarative rules)
		if riskEngine != nil {
			metrics, err := riskEngine.GetMetrics(ctx, userID)
			if err != nil {
				// Log error but continue for MVP
			}
			if triggered, msg := riskEngine.EvaluateRules(ctx, userID, metrics); triggered {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Risk control triggered: " + msg})
				return
			}
		}

		// 2. Pre-check Balance
		balance, err := walletSvc.CheckBalance(ctx, userID)
		if err != nil {
			if err == billing.ErrWalletNotFound {
				c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{"error": "Wallet not initialized"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to check balance"})
			return
		}
		if balance <= 0 {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{"error": "Insufficient balance. Please recharge."})
			return
		}

		c.Set("user_id", userID)
		c.Set("key_hash", keyHash)
		c.Next()
	}
}

func hashAPIKey(key string) string {
	h := sha256.New()
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}
