package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/synora/synora-gateway/internal/billing"
	"github.com/synora/synora-gateway/internal/storage/pg"
)

type StripeHandler struct {
	wallet *billing.WalletService
}

func NewStripeHandler() *StripeHandler {
	return &StripeHandler{
		wallet: billing.NewWalletService(),
	}
}

// Webhook handles Stripe events
func (h *StripeHandler) Webhook(c *gin.Context) {
	// In production, verify Stripe signature using stripe-go/webhook.ConstructEvent
	// For MVP, we'll parse the event body directly (ensure webhook secret is set)
	
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var event struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				Metadata map[string]string `json:"metadata"`
				Amount   int64             `json:"amount"` // in cents
				Currency string            `json:"currency"`
			} `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse event"})
		return
	}

	if event.Type == "checkout.session.completed" {
		userIDStr := event.Data.Object.Metadata["user_id"]
		userID, _ := strconv.Atoi(userIDStr)
		amountCNY := float64(event.Data.Object.Amount) / 100.0 // Assuming payment is in CNY cents

		log.Printf("Stripe: Payment received for User %d: %.2f CNY", userID, amountCNY)

		// 1. Get Wallet ID
		db := pg.GetDB()
		var walletID int
		err := db.QueryRow(c.Request.Context(), "SELECT id FROM wallets WHERE user_id = $1", userID).Scan(&walletID)
		if err != nil {
			log.Printf("Stripe Error: failed to find wallet for user %d: %v", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Wallet not found"})
			return
		}

		// 2. Perform Recharge (Update Balance and Log Transaction)
		tx, err := db.Begin(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
			return
		}
		defer func() { _ = tx.Rollback(c.Request.Context()) }()

		_, err = tx.Exec(c.Request.Context(), "UPDATE wallets SET balance = balance + $1, updated_at = NOW() WHERE id = $2", amountCNY, walletID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balance"})
			return
		}

		_, err = tx.Exec(c.Request.Context(), `
			INSERT INTO wallet_transactions (wallet_id, amount, type, description)
			VALUES ($1, $2, 'recharge', 'Stripe Payment')
		`, walletID, amountCNY)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log transaction"})
			return
		}

		if err := tx.Commit(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit"})
			return
		}
		
		log.Printf("Stripe Success: User %d recharged %.2f CNY", userID, amountCNY)
	}

	c.Status(http.StatusOK)
}

func (h *StripeHandler) CreateCheckoutSession(c *gin.Context) {
	// Placeholder for creating a checkout session
	// This would typically involve calling Stripe API and returning a session URL
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Please use Stripe Dashboard to create test events for MVP"})
}
