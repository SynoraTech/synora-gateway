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
	"github.com/synora/synora-gateway/internal/storage/pg"
	"github.com/synora/synora-gateway/internal/storage/redis"
)

const (
	AuthHeader   = "Authorization"
	BearerPrefix = "Bearer "
)

// AuthMiddleware handles API Key authentication and initial balance check
func AuthMiddleware() gin.HandlerFunc {
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

		// 1. Hash the key
		keyHash := hashAPIKey(apiKey)

		// 2. Check Redis Cache for Auth
		ctx := c.Request.Context()
		rdb := redis.GetRedis()
		authCacheKey := fmt.Sprintf("auth:key:%s", keyHash)
		
		userIDStr, err := rdb.Get(ctx, authCacheKey).Result()
		var userID int
		if err == nil && userIDStr != "" {
			userID, _ = strconv.Atoi(userIDStr)
		} else {
			// 3. Database lookup
			db := pg.GetDB()
			var status string
			query := "SELECT user_id, status FROM api_keys WHERE key_hash = $1"
			err = db.QueryRow(ctx, query, keyHash).Scan(&userID, &status)
			
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
				return
			}

			if status != "active" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API key is " + status})
				return
			}

			// Update Auth Cache
			rdb.Set(ctx, authCacheKey, fmt.Sprintf("%d", userID), 300*1e9)
		}

		// 4. Pre-check Balance (must be > 0)
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
