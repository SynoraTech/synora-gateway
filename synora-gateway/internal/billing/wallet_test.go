package billing

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	storageRedis "github.com/synora/synora-gateway/internal/storage/redis"
)

func TestWalletService_RedisCache(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	storageRedis.SetRedisForTest(client)
	defer storageRedis.SetRedisForTest(nil)

	svc := NewWalletService()
	userID := 123
	expectedBalance := 100.50

	// Set value in Redis
	_ = mr.Set(fmt.Sprintf("wallet:balance:%d", userID), "100.50")

	balance, err := svc.CheckBalance(context.Background(), userID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if balance != expectedBalance {
		t.Errorf("Expected balance %.2f, got %.2f", expectedBalance, balance)
	}
}
