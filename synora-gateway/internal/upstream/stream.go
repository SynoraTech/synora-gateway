package upstream

import (
	"bufio"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/synora/synora-gateway/internal/adapter"
)

// StreamForwarder handles forwarding SSE stream from upstream to client
type StreamForwarder struct{}

func NewStreamForwarder() *StreamForwarder {
	return &StreamForwarder{}
}

// Forward streams the upstream response body to the Gin context
func (f *StreamForwarder) Forward(c *gin.Context, resp *http.Response, adp adapter.ProviderAdapter) (int, error) {
	// Set outgoing headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Use bufio.Reader to read lines (SSE data)
	reader := bufio.NewReader(resp.Body)
	totalBytes := 0

	c.Stream(func(w io.Writer) bool {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return false
			}
			return false
		}

		// Convert chunk using adapter
		converted, err := adp.ConvertStreamChunk(line)
		if err != nil {
			// If conversion fails, we might still want to try sending the raw line or stop
			return false
		}

		totalBytes += len(converted)
		w.Write(converted)

		// Check if it's the end of stream using adapter
		if adp.IsStreamEnd(line) {
			return false
		}

		return true
	})

	return totalBytes, nil
}

// ProxyResponse copies headers and status from upstream to client (for non-streaming)
func (f *StreamForwarder) ProxyResponse(c *gin.Context, resp *http.Response) {
	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Header(k, v)
		}
	}
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
