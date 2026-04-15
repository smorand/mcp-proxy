package oauth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/smorand/mcp-proxy/internal/errors"
)

// OAuthDiscovery represents the OAuth 2.1 discovery document
type OAuthDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	Issuer                string `json:"issuer"`
}

// DiscoverEndpoints fetches the OAuth 2.1 discovery document from the MCP server
// It looks for .well-known/oauth-authorization-server at the server URL
func DiscoverEndpoints(serverURL string) (*OAuthDiscovery, error) {
	// Parse the server URL
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, errors.NewConfigError("invalid server URL", err)
	}

	// Construct the discovery URL
	discoveryURL := fmt.Sprintf("%s://%s/.well-known/oauth-authorization-server", parsedURL.Scheme, parsedURL.Host)

	// Create HTTP client with timeout and TLS config for testing
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Fetch the discovery document
	resp, err := client.Get(discoveryURL)
	if err != nil {
		return nil, errors.NewNetworkError(
			fmt.Sprintf("failed to fetch OAuth discovery document from %s", discoveryURL),
			err,
		)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewNetworkError(
			fmt.Sprintf("OAuth discovery endpoint returned status %d", resp.StatusCode),
			nil,
		)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewNetworkError("failed to read OAuth discovery response", err)
	}

	// Parse JSON
	var discovery OAuthDiscovery
	if err := json.Unmarshal(body, &discovery); err != nil {
		return nil, errors.NewNetworkError("failed to parse OAuth discovery document", err)
	}

	// Validate required fields
	if discovery.AuthorizationEndpoint == "" {
		return nil, errors.NewNetworkError("OAuth discovery document missing authorization_endpoint", nil)
	}
	if discovery.TokenEndpoint == "" {
		return nil, errors.NewNetworkError("OAuth discovery document missing token_endpoint", nil)
	}

	return &discovery, nil
}
