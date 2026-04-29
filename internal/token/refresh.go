package token

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/smorand/mcp-proxy/internal/apperr"
)

// RefreshResponse holds the OAuth refresh-token response payload.
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// ErrRefreshRejected indicates the refresh token was rejected (HTTP 400/401).
// Callers should fall back to the full OAuth flow.
var ErrRefreshRejected = errors.New("refresh token rejected")

// RefreshAccessToken exchanges a refresh token for a fresh access token.
// The provided HTTP client controls TLS behaviour and timeouts.
// Returns an error wrapping ErrRefreshRejected on 400/401 responses.
func RefreshAccessToken(
	ctx context.Context,
	client *http.Client,
	tokenEndpoint string,
	clientID string,
	clientSecret string,
	refreshToken string,
) (*RefreshResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, apperr.NewNetworkError("failed to create token refresh request", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, apperr.NewNetworkError(
			fmt.Sprintf("Failed to refresh token: %v", err),
			err,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperr.NewNetworkError("failed to read token refresh response", err)
	}

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: server returned %d", ErrRefreshRejected, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperr.NewAuthError(
			fmt.Sprintf("token refresh endpoint returned status %d", resp.StatusCode),
			nil,
		)
	}

	var tokenResp RefreshResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperr.NewNetworkError("failed to parse token refresh response", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, apperr.NewAuthError("token refresh response missing access_token", nil)
	}
	if tokenResp.ExpiresIn == 0 {
		return nil, apperr.NewAuthError("token refresh response missing expires_in", nil)
	}

	return &tokenResp, nil
}
