package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/smorand/mcp-proxy/internal/config"
	"github.com/smorand/mcp-proxy/internal/errors"
	"github.com/smorand/mcp-proxy/internal/oauth"
	"github.com/smorand/mcp-proxy/internal/token"
)

func main() {
	// Parse configuration
	cfg, err := config.Parse()
	if err != nil {
		errors.Fatal(err)
	}

	// Create token storage
	storage, err := token.NewStorage()
	if err != nil {
		errors.Fatal(err)
	}

	// Try to load existing token
	tokenData, err := storage.Load(cfg.ServerURL)
	if err == nil && time.Now().Before(tokenData.ExpirationTime) {
		// Token exists and is valid
		fmt.Printf("Using cached token for %s\n", cfg.ServerURL)
		// TODO: US-004 will implement MCP proxy using this token
		return
	}

	// No valid token, start OAuth flow
	fmt.Printf("Starting OAuth flow for %s\n", cfg.ServerURL)

	// Step 1: Discover OAuth endpoints
	discovery, err := oauth.DiscoverEndpoints(cfg.ServerURL)
	if err != nil {
		errors.Fatal(err)
	}

	// Step 2: Generate PKCE codes
	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		errors.Fatal(errors.NewAuthError("failed to generate PKCE codes", err))
	}

	// Step 3: Start callback server
	callbackServer, err := oauth.NewCallbackServer()
	if err != nil {
		errors.Fatal(err)
	}

	redirectURI, err := callbackServer.Start()
	if err != nil {
		errors.Fatal(err)
	}
	defer callbackServer.Stop()

	// Step 4: Build authorization URL
	authURL := buildAuthorizationURL(
		discovery.AuthorizationEndpoint,
		cfg.ClientID,
		redirectURI,
		pkce.CodeChallenge,
	)

	// Step 5: Open browser
	fmt.Printf("Opening browser for authorization...\n")
	if err := oauth.OpenBrowser(authURL); err != nil {
		errors.Fatal(err)
	}

	// Step 6: Wait for callback (5 minute timeout)
	fmt.Printf("Waiting for authorization callback...\n")
	code, err := callbackServer.WaitForCallback(5 * time.Minute)
	if err != nil {
		errors.Fatal(err)
	}

	// Step 7: Exchange code for tokens
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
		errors.Fatal(err)
	}

	// Step 8: Save token
	if err := storage.Save(cfg.ServerURL, tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn); err != nil {
		errors.Fatal(err)
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