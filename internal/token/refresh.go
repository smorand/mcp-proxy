package token

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/smorand/mcp-proxy/internal/errors"
)

// RefreshResponse represents the OAuth token refresh response.
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// ErrRefreshRejected indicates the refresh token was rejected by the server (400/401).
// This is not retryable; the caller should fall back to full OAuth flow.
var ErrRefreshRejected = fmt.Errorf("refresh token rejected")

// RefreshAccessToken attempts to refresh an expired access token using the refresh token.
// It sends a POST request to the token endpoint with grant_type=refresh_token.
// Returns the new token response on success.
// Returns ErrRefreshRejected (wrapped) if the server rejects the refresh token (400/401).
// Returns a network error for connection failures or timeouts.
func RefreshAccessToken(
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

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", tokenEndpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, errors.NewNetworkError("failed to create token refresh request", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.NewNetworkError(
			fmt.Sprintf("Failed to refresh token: %v", err),
			err,
		)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewNetworkError("failed to read token refresh response", err)
	}

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: server returned %d", ErrRefreshRejected, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewAuthError(
			fmt.Sprintf("token refresh endpoint returned status %d", resp.StatusCode),
			nil,
		)
	}

	var tokenResp RefreshResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, errors.NewNetworkError("failed to parse token refresh response", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, errors.NewAuthError("token refresh response missing access_token", nil)
	}
	if tokenResp.ExpiresIn == 0 {
		return nil, errors.NewAuthError("token refresh response missing expires_in", nil)
	}

	return &tokenResp, nil
}
