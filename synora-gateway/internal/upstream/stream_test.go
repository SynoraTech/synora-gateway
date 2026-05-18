package upstream

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/synora/synora-gateway/internal/adapter/openai"
)

type closeNotifyingRecorder struct {
	*httptest.ResponseRecorder
}

func (c *closeNotifyingRecorder) CloseNotify() <-chan bool {
	return make(chan bool)
}

func TestStreamForwarder_ProxyResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	upstreamResp := &http.Response{
		StatusCode: http.StatusCreated,
		Header: http.Header{
			"X-Test": []string{"Value"},
		},
		Body: io.NopCloser(bytes.NewBufferString("Hello World")),
	}

	f := NewStreamForwarder()
	body := f.ProxyResponse(c, upstreamResp)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
	if w.Header().Get("X-Test") != "Value" {
		t.Errorf("Expected header X-Test: Value, got %s", w.Header().Get("X-Test"))
	}
	if string(body) != "Hello World" {
		t.Errorf("Expected body Hello World, got %s", string(body))
	}
}

func TestStreamForwarder_Forward(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &closeNotifyingRecorder{httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(w)

	// Mock SSE stream
	sseData := "data: {\"choices\": [{\"delta\": {\"content\": \"Hello\"}}]}\n\ndata: {\"choices\": [{\"delta\": {\"content\": \" World\"}}]}\n\ndata: [DONE]\n\n"
	upstreamResp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString(sseData)),
	}

	f := NewStreamForwarder()
	adp := openai.NewOpenAIAdapter()

	// Forward is a blocking call that uses c.Stream
	// We need to run it and then check the recorder
	res, err := f.Forward(c, upstreamResp, adp)

	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	if res.GeneratedText != "Hello World" {
		t.Errorf("Expected generated text 'Hello World', got '%s'", res.GeneratedText)
	}

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}
