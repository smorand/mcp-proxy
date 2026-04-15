package oauth

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

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// ExchangeCodeForToken exchanges an authorization code for access and refresh tokens
func ExchangeCodeForToken(
	tokenEndpoint string,
	clientID string,
	clientSecret string,
	code string,
	redirectURI string,
	codeVerifier string,
) (*TokenResponse, error) {
	// Prepare form data
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code_verifier", codeVerifier)

	// Create HTTP client with timeout and TLS config for testing
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Create request
	req, err := http.NewRequest("POST", tokenEndpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, errors.NewNetworkError("failed to create token exchange request", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.NewNetworkError(
			fmt.Sprintf("failed to exchange authorization code: %v", err),
			err,
		)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewNetworkError("failed to read token response", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		// Try to parse error response
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error != "" {
			errorMsg := errorResp.Error
			if errorResp.ErrorDescription != "" {
				errorMsg = fmt.Sprintf("%s: %s", errorMsg, errorResp.ErrorDescription)
			}
			return nil, errors.NewAuthError(
				fmt.Sprintf("Failed to exchange authorization code for tokens: %s", errorMsg),
				nil,
			)
		}
		return nil, errors.NewAuthError(
			fmt.Sprintf("token endpoint returned status %d", resp.StatusCode),
			nil,
		)
	}

	// Parse token response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, errors.NewNetworkError("failed to parse token response", err)
	}

	// Validate required fields
	if tokenResp.AccessToken == "" {
		return nil, errors.NewAuthError("token response missing access_token", nil)
	}
	if tokenResp.ExpiresIn == 0 {
		return nil, errors.NewAuthError("token response missing expires_in", nil)
	}

	return &tokenResp, nil
}
