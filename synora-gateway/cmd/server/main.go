package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/synora/synora-gateway/internal/api"
	"github.com/synora/synora-gateway/internal/api/middleware"
	"github.com/synora/synora-gateway/internal/audit"
	"github.com/synora/synora-gateway/internal/billing"
	"github.com/synora/synora-gateway/internal/risk"
	"github.com/synora/synora-gateway/internal/router"
	"github.com/synora/synora-gateway/internal/storage/clickhouse"
	"github.com/synora/synora-gateway/internal/storage/pg"
	"github.com/synora/synora-gateway/internal/storage/redis"
)

func main() {
	// ... (Existing init logic)
	mode := os.Getenv("GIN_MODE")
	if mode == "" {
		mode = gin.DebugMode
	}
	gin.SetMode(mode)

	// 2. Initialize Storage and Services
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://synora:synora123@localhost:5432/synora?sslmode=disable"
	}
	if _, err := pg.InitDB(dbURL); err != nil {
		log.Fatalf("Failed to initialize PostgreSQL: %v", err)
	}
	defer pg.CloseDB()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	if _, err := redis.InitRedis(redisURL); err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer redis.CloseRedis()

	chAddr := os.Getenv("CLICKHOUSE_ADDR")
	if chAddr == "" {
		chAddr = "localhost:9000"
	}
	if _, err := clickhouse.InitClickHouse(chAddr, "synora", "synora", "synora123"); err != nil {
		log.Printf("Warning: Failed to initialize ClickHouse: %v", err)
	}

	// Risk Engine Init
	riskEngine, err := risk.NewRiskEngine("deploy/risk_rules.yaml", []string{"sensitive_word_1", "illegal_content_token"})
	if err != nil {
		log.Printf("Warning: Failed to initialize Risk Engine: %v", err)
	}

	hm := router.NewHealthManager()
	cs := router.NewChannelService(hm)
	if err := cs.LoadAllChannels(context.Background()); err != nil {
		log.Printf("Warning: Failed to load channels: %v", err)
	}

	logger := audit.NewLogDispatcher(1000)
	chatHandler := api.NewChatHandler(hm, cs, logger)

	// 3. Setup Router
	r := gin.Default()

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "v1.0.0-p0"})
	})

	// V1 API group with Authentication and Risk Control
	v1 := r.Group("/v1")
	v1.Use(middleware.AuthMiddleware(riskEngine))
	{
		v1.POST("/chat/completions", chatHandler.ChatCompletions)
		
		// Anthropic native messages endpoint
		v1.POST("/messages", chatHandler.AnthropicMessages)

		v1.GET("/ping", func(c *gin.Context) {
			userID, _ := c.Get("user_id")
			c.JSON(http.StatusOK, gin.H{"message": "pong", "user_id": userID})
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Synora Gateway starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
