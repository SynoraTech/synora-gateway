package billing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/synora/synora-gateway/internal/storage/pg"
	"github.com/synora/synora-gateway/internal/storage/redis"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrWalletNotFound      = errors.New("wallet not found")
)

// WalletService handles balance checks and deductions
type WalletService struct{}

func NewWalletService() *WalletService {
	return &WalletService{}
}

// CheckBalance checks if the user has enough balance in Redis cache first
func (s *WalletService) CheckBalance(ctx context.Context, userID int) (float64, error) {
	rdb := redis.GetRedis()
	cacheKey := fmt.Sprintf("wallet:balance:%d", userID)

	// 1. Try Redis Cache
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		balance, _ := strconv.ParseFloat(val, 64)
		return balance, nil
	}

	// 2. Fallback to DB
	db := pg.GetDB()
	var balance float64
	query := "SELECT balance FROM wallets WHERE user_id = $1"
	err = db.QueryRow(ctx, query, userID).Scan(&balance)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, ErrWalletNotFound
		}
		return 0, err
	}

	// 3. Update Cache (5 seconds TTL as per plan)
	rdb.Set(ctx, cacheKey, fmt.Sprintf("%.4f", balance), 5*time.Second)

	return balance, nil
}

// DeductBalance performs a thread-safe deduction using PostgreSQL pessimistic locking
func (s *WalletService) DeductBalance(ctx context.Context, userID int, amount float64, requestID string) error {
	if amount <= 0 {
		return nil
	}

	db := pg.GetDB()
	
	// Start transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Lock and Get current balance
	var walletID int
	var balance float64
	query := "SELECT id, balance FROM wallets WHERE user_id = $1 FOR UPDATE"
	err = tx.QueryRow(ctx, query, userID).Scan(&walletID, &balance)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrWalletNotFound
		}
		return err
	}

	// 2. Check if enough
	if balance < amount {
		return ErrInsufficientBalance
	}

	// 3. Update balance
	newBalance := balance - amount
	updateQuery := "UPDATE wallets SET balance = $1, updated_at = NOW() WHERE id = $2"
	_, err = tx.Exec(ctx, updateQuery, newBalance, walletID)
	if err != nil {
		return err
	}

	// 4. Record transaction
	logQuery := `
		INSERT INTO wallet_transactions (wallet_id, amount, type, request_id, description)
		VALUES ($1, $2, 'consume', $3, 'API Usage Deduction')
	`
	_, err = tx.Exec(ctx, logQuery, walletID, -amount, requestID)
	if err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// 5. Invalidate Redis cache to ensure consistency
	rdb := redis.GetRedis()
	cacheKey := fmt.Sprintf("wallet:balance:%d", userID)
	rdb.Del(ctx, cacheKey)

	return nil
}
