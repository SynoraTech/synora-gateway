package upstream

import (
	"bufio"
	"bytes"
	"encoding/json"
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

// StreamResponse represents the returned data from stream
type StreamResponse struct {
	TotalBytes    int
	GeneratedText string
}

// Forward streams the upstream response body to the Gin context
func (f *StreamForwarder) Forward(c *gin.Context, resp *http.Response, adp adapter.ProviderAdapter) (*StreamResponse, error) {
	// Set outgoing headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Use bufio.Reader to read lines (SSE data)
	reader := bufio.NewReader(resp.Body)
	totalBytes := 0
	var textBuf bytes.Buffer

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

		// Parse the converted output (which is in OpenAI SSE format) to extract generated text
		if bytes.HasPrefix(converted, []byte("data: ")) && !bytes.HasPrefix(converted, []byte("data: [DONE]")) {
			dataJSON := bytes.TrimPrefix(converted, []byte("data: "))
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(dataJSON, &chunk); err == nil {
				if len(chunk.Choices) > 0 {
					textBuf.WriteString(chunk.Choices[0].Delta.Content)
				}
			}
		}

		// Check if it's the end of stream using adapter
		if adp.IsStreamEnd(line) {
			return false
		}

		return true
	})

	return &StreamResponse{
		TotalBytes:    totalBytes,
		GeneratedText: textBuf.String(),
	}, nil
}

// ProxyResponse copies headers and status from upstream to client (for non-streaming)
// Returns the response body bytes so we can parse it for usage if needed.
func (f *StreamForwarder) ProxyResponse(c *gin.Context, resp *http.Response) []byte {
	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Header(k, v)
		}
	}
	c.Status(resp.StatusCode)
	
	bodyBytes, _ := io.ReadAll(resp.Body)
	c.Writer.Write(bodyBytes)
	return bodyBytes
}
