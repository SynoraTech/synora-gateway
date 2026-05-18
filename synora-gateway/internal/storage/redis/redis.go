package redis

import (
	"context"
	"log"
	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	client *redis.Client
	once   sync.Once
)

// InitRedis initializes the Redis client
func InitRedis(url string) (*redis.Client, error) {
	var err error
	once.Do(func() {
		opt, parseErr := redis.ParseURL(url)
		if parseErr != nil {
			err = parseErr
			return
		}

		client = redis.NewClient(opt)

		// Ping Redis to ensure connection
		ctx := context.Background()
		if _, pingErr := client.Ping(ctx).Result(); pingErr != nil {
			err = pingErr
			return
		}

		log.Println("Redis client initialized successfully")
	})

	return client, err
}

// GetRedis returns the initialized Redis client
func GetRedis() *redis.Client {
	if client == nil {
		log.Fatal("Redis client has not been initialized. Call InitRedis first.")
	}
	return client
}

// CloseRedis closes the Redis client
func CloseRedis() {
	if client != nil {
		client.Close()
	}
}
