package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/synora/synora-gateway/internal/api"
	"github.com/synora/synora-gateway/internal/api/middleware"
	"github.com/synora/synora-gateway/internal/audit"
	"github.com/synora/synora-gateway/internal/router"
	"github.com/synora/synora-gateway/internal/storage/clickhouse"
	"github.com/synora/synora-gateway/internal/storage/pg"
	"github.com/synora/synora-gateway/internal/storage/redis"
)

func main() {
	// ... (Load environment and set Gin mode)
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

	hm := router.NewHealthManager()
	logger := audit.NewLogDispatcher(1000)
	chatHandler := api.NewChatHandler(hm, logger)

	// 3. Setup Router
	r := gin.Default()

	// ... (Metrics and Health check)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": "v1.0.0-mvp",
		})
	})

	// V1 API group with Authentication
	v1 := r.Group("/v1")
	v1.Use(middleware.AuthMiddleware())
	{
		v1.POST("/chat/completions", chatHandler.ChatCompletions)
		
		v1.GET("/ping", func(c *gin.Context) {
			userID, _ := c.Get("user_id")
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
				"user_id": userID,
			})
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
