package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthMiddleware_NoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)

	handler := AuthMiddleware(nil)
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.Header.Set("Authorization", "Basic 123")

	handler := AuthMiddleware(nil)
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "sk-123456"
	hash := hashAPIKey(key)
	if hash == "" {
		t.Errorf("Hash should not be empty")
	}
	if len(hash) != 64 { // SHA-256 hex is 64 chars
		t.Errorf("Expected length 64, got %d", len(hash))
	}
}
