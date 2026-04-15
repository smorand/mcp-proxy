package main

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/smorand/mcp-proxy/internal/config"
	apperrors "github.com/smorand/mcp-proxy/internal/errors"
	"github.com/smorand/mcp-proxy/internal/oauth"
	"github.com/smorand/mcp-proxy/internal/token"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		apperrors.Fatal(err)
	}

	storage, err := token.NewStorage()
	if err != nil {
		apperrors.Fatal(err)
	}

	// Try to load existing token
	tokenData, err := storage.Load(cfg.ServerURL)
	if err == nil && !tokenData.IsExpired(time.Now()) {
		fmt.Printf("Using cached token for %s\n", cfg.ServerURL)
		// TODO: US-004 will implement MCP proxy using this token
		return
	}

	// Token expired: attempt refresh if we have a refresh token
	if err == nil && tokenData.IsExpired(time.Now()) && tokenData.HasRefreshToken() {
		fmt.Printf("Token expired, attempting refresh...\n")

		discovery, discErr := oauth.DiscoverEndpoints(cfg.ServerURL)
		if discErr != nil {
			apperrors.Fatal(discErr)
		}

		refreshResp, refreshErr := token.RefreshAccessToken(
			discovery.TokenEndpoint,
			cfg.ClientID,
			cfg.ClientSecret,
			tokenData.RefreshToken,
		)

		if refreshErr == nil {
			newRefresh := refreshResp.RefreshToken
			if newRefresh == "" {
				newRefresh = tokenData.RefreshToken
			}
			if saveErr := storage.Save(cfg.ServerURL, refreshResp.AccessToken, newRefresh, refreshResp.ExpiresIn); saveErr != nil {
				apperrors.Fatal(saveErr)
			}
			expirationTime := time.Now().Add(time.Duration(refreshResp.ExpiresIn) * time.Second)
			fmt.Printf("Token refreshed successfully. Expires at: %s\n", expirationTime.Format(time.RFC3339))
			// TODO: US-004 will implement MCP proxy using this token
			return
		}

		// Refresh rejected (400/401): fall back to full OAuth flow
		if errors.Is(refreshErr, token.ErrRefreshRejected) {
			fmt.Printf("Refresh token rejected, starting full OAuth flow...\n")
		} else {
			// Network or other error during refresh
			apperrors.Fatal(refreshErr)
		}
	}

	// Full OAuth flow (first time, missing refresh token, or refresh rejected)
	fmt.Printf("Starting OAuth flow for %s\n", cfg.ServerURL)
	performFullOAuthFlow(cfg, storage)
}

// performFullOAuthFlow runs the complete OAuth2.1 authorization code flow with PKCE.
func performFullOAuthFlow(cfg *config.Config, storage *token.Storage) {
	discovery, err := oauth.DiscoverEndpoints(cfg.ServerURL)
	if err != nil {
		apperrors.Fatal(err)
	}

	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		apperrors.Fatal(apperrors.NewAuthError("failed to generate PKCE codes", err))
	}

	callbackServer, err := oauth.NewCallbackServer()
	if err != nil {
		apperrors.Fatal(err)
	}

	redirectURI, err := callbackServer.Start()
	if err != nil {
		apperrors.Fatal(err)
	}
	defer callbackServer.Stop()

	authURL := buildAuthorizationURL(
		discovery.AuthorizationEndpoint,
		cfg.ClientID,
		redirectURI,
		pkce.CodeChallenge,
	)

	fmt.Printf("Opening browser for authorization...\n")
	if err := oauth.OpenBrowser(authURL); err != nil {
		apperrors.Fatal(err)
	}

	fmt.Printf("Waiting for authorization callback...\n")
	code, err := callbackServer.WaitForCallback(5 * time.Minute)
	if err != nil {
		apperrors.Fatal(err)
	}

	fmt.Printf("Exchanging authorization code for tokens...\n")
	tokenResp, err := oauth.ExchangeCodeForToken(
		discovery.TokenEndpoint,
		cfg.ClientID,
		cfg.ClientSecret,
		code,
		redirectURI,
		pkce.CodeVerifier,
	)
	if err != nil {
		apperrors.Fatal(err)
	}

	if err := storage.Save(cfg.ServerURL, tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn); err != nil {
		apperrors.Fatal(err)
	}

	expirationTime := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	fmt.Printf("Token saved successfully. Expires at: %s\n", expirationTime.Format(time.RFC3339))
	// TODO: US-004 will implement MCP proxy using this token
}

// buildAuthorizationURL constructs the OAuth authorization URL with PKCE
func buildAuthorizationURL(authEndpoint, clientID, redirectURI, codeChallenge string) string {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	// Note: scope is optional and server-dependent, not included by default

	return fmt.Sprintf("%s?%s", authEndpoint, params.Encode())
}
