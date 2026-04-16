package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/smorand/mcp-proxy/internal/errors"
)

// MCPClient handles HTTP communication with a remote MCP server.
type MCPClient struct {
	serverURL  string
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
	Body       io.ReadCloser
	StatusCode int
}

// Forward sends a JSON-RPC message to the MCP server with Bearer token auth.
// The caller is responsible for closing Body in the returned ForwardResult.
func (c *MCPClient) Forward(ctx context.Context, message []byte, accessToken string) (*ForwardResult, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.serverURL, bytes.NewReader(message))
	if err != nil {
		return nil, errors.NewNetworkError("failed to create MCP request", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.NewNetworkError(
			fmt.Sprintf("Failed to connect to MCP server: %v", err),
			err,
		)
	}

	return &ForwardResult{
		Body:       resp.Body,
		StatusCode: resp.StatusCode,
	}, nil
}
