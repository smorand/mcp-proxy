package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/smorand/mcp-proxy/internal/apperr"
)

// TokenResponse holds an OAuth 2.1 token endpoint successful response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// ExchangeCodeForToken exchanges an authorization code for access and
// refresh tokens. The provided HTTP client controls TLS behaviour and
// timeouts.
func ExchangeCodeForToken(
	ctx context.Context,
	client *http.Client,
	tokenEndpoint string,
	clientID string,
	clientSecret string,
	code string,
	redirectURI string,
	codeVerifier string,
) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, apperr.NewNetworkError("failed to create token exchange request", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, apperr.NewNetworkError(
			fmt.Sprintf("failed to exchange authorization code: %v", err),
			err,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperr.NewNetworkError("failed to read token response", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error != "" {
			msg := errorResp.Error
			if errorResp.ErrorDescription != "" {
				msg = fmt.Sprintf("%s: %s", msg, errorResp.ErrorDescription)
			}
			return nil, apperr.NewAuthError(
				fmt.Sprintf("Failed to exchange authorization code for tokens: %s", msg),
				nil,
			)
		}
		return nil, apperr.NewAuthError(
			fmt.Sprintf("token endpoint returned status %d", resp.StatusCode),
			nil,
		)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperr.NewNetworkError("failed to parse token response", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, apperr.NewAuthError("token response missing access_token", nil)
	}
	if tokenResp.ExpiresIn == 0 {
		return nil, apperr.NewAuthError("token response missing expires_in", nil)
	}

	return &tokenResp, nil
}
