package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/smorand/mcp-proxy/internal/errors"
)

// MCPClient handles HTTP communication with a remote MCP server.
type MCPClient struct {
	serverURL  string
	sessionID  string
	httpClient *http.Client
}

// NewMCPClient creates an HTTP client configured for the given MCP server URL.
func NewMCPClient(serverURL string) *MCPClient {
	return &MCPClient{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// ForwardResult contains the MCP server HTTP response.
type ForwardResult struct {
	Body        io.ReadCloser
	StatusCode  int
	ContentType string
	SessionID   string
}

// Forward sends a JSON-RPC message to the MCP server with Bearer token auth.
// The caller is responsible for closing Body in the returned ForwardResult.
func (c *MCPClient) Forward(ctx context.Context, message []byte, accessToken string) (*ForwardResult, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.serverURL, bytes.NewReader(message))
	if err != nil {
		return nil, errors.NewNetworkError("failed to create MCP request", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.NewNetworkError(
			fmt.Sprintf("Failed to connect to MCP server: %v", err),
			err,
		)
	}

	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID != "" {
		c.sessionID = sessionID
	}

	return &ForwardResult{
		Body:        resp.Body,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		SessionID:   sessionID,
	}, nil
}

// WriteSSEDataTo reads an SSE stream, extracts JSON from "data:" lines, and
// writes each one immediately to w. Returns after all data lines are written.
// The response body is closed in the background to avoid blocking on slow streams.
func WriteSSEDataTo(w io.Writer, body io.ReadCloser) error {
	scanner := bufio.NewScanner(body)
	var wrote bool
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := line[6:]
			if _, err := w.Write([]byte(data + "\n")); err != nil {
				body.Close()
				return err
			}
			wrote = true
		} else if wrote && line == "" {
			// Empty line after data = end of SSE event; stop reading
			break
		}
	}
	// Close body in background to avoid blocking on chunked streams
	go body.Close()
	return scanner.Err()
}
