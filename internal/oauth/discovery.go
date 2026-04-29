package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/smorand/mcp-proxy/internal/apperr"
)

// Discovery represents the OAuth 2.1 authorization server metadata.
type Discovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	Issuer                string `json:"issuer"`
}

// DiscoverEndpoints fetches the OAuth 2.1 authorization server metadata
// from `<server>/.well-known/oauth-authorization-server`. The provided
// HTTP client is used for the request; pass http.DefaultClient unless TLS
// behaviour needs to be customised (e.g. tests against httptest TLS servers).
func DiscoverEndpoints(ctx context.Context, serverURL string, client *http.Client) (*Discovery, error) {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, apperr.NewConfigError("invalid server URL", err)
	}

	discoveryURL := fmt.Sprintf("%s://%s/.well-known/oauth-authorization-server", parsedURL.Scheme, parsedURL.Host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, apperr.NewNetworkError("failed to create OAuth discovery request", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, apperr.NewNetworkError(
			fmt.Sprintf("failed to fetch OAuth discovery document from %s", discoveryURL),
			err,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, apperr.NewNetworkError(
			fmt.Sprintf("OAuth discovery endpoint returned status %d", resp.StatusCode),
			nil,
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperr.NewNetworkError("failed to read OAuth discovery response", err)
	}

	var discovery Discovery
	if err := json.Unmarshal(body, &discovery); err != nil {
		return nil, apperr.NewNetworkError("failed to parse OAuth discovery document", err)
	}

	if discovery.AuthorizationEndpoint == "" {
		return nil, apperr.NewNetworkError("OAuth discovery document missing authorization_endpoint", nil)
	}
	if discovery.TokenEndpoint == "" {
		return nil, apperr.NewNetworkError("OAuth discovery document missing token_endpoint", nil)
	}

	return &discovery, nil
}
