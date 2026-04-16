package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/smorand/mcp-proxy/internal/config"
	apperrors "github.com/smorand/mcp-proxy/internal/errors"
	"github.com/smorand/mcp-proxy/internal/oauth"
	"github.com/smorand/mcp-proxy/internal/proxy"
	"github.com/smorand/mcp-proxy/internal/telemetry"
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

	// Initialize telemetry (JSONL trace file)
	tracer, err := initTelemetry(storage.GetCacheDir())
	if err != nil {
		apperrors.Fatal(err)
	}
	if tracer != nil {
		defer tracer.Close()
	}

	// Obtain a valid access token (cache, refresh, or full OAuth flow)
	accessToken, err := getValidToken(cfg, storage, tracer)
	if err != nil {
		apperrors.Fatal(err)
	}

	// Graceful shutdown on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Run the MCP proxy (stdin → HTTP → stdout)
	handler := proxy.NewHandler(proxy.HandlerConfig{
		ServerURL:    cfg.ServerURL,
		Storage:      storage,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Tracer:       tracer,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		AccessToken:  accessToken,
	})

	if err := handler.Run(ctx); err != nil {
		apperrors.Fatal(err)
	}
}

// initTelemetry creates a JSONL tracer writing to ~/.cache/mcp-proxy/traces.jsonl.
func initTelemetry(cacheDir string) (*telemetry.Tracer, error) {
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, apperrors.NewFileSystemError(
			fmt.Sprintf("Cannot create cache directory at %s", cacheDir),
			err,
		)
	}
	tracePath := filepath.Join(cacheDir, "traces.jsonl")
	tracer, err := telemetry.NewTracer(tracePath, "mcp-proxy")
	if err != nil {
		return nil, apperrors.NewFileSystemError("failed to initialize telemetry", err)
	}
	return tracer, nil
}

// getValidToken returns a valid access token, refreshing or re-authenticating as needed.
func getValidToken(cfg *config.Config, storage *token.Storage, tracer *telemetry.Tracer) (string, error) {
	// Trace the token validation step
	if tracer != nil {
		_, span := tracer.StartSpan(context.Background(), "oauth.token_check",
			telemetry.StringAttr("oauth.flow.step", "token_validation"),
			telemetry.StringAttr("mcp.server.url", cfg.ServerURL),
		)
		defer span.End(nil)
	}

	// Try to load cached token
	tokenData, err := storage.Load(cfg.ServerURL)
	if err == nil && !tokenData.IsExpired(time.Now()) {
		return tokenData.AccessToken, nil
	}

	// Token expired: attempt refresh if refresh_token is available
	if err == nil && tokenData.IsExpired(time.Now()) && tokenData.HasRefreshToken() {
		discovery, discErr := oauth.DiscoverEndpoints(cfg.ServerURL)
		if discErr != nil {
			return "", discErr
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
				return "", saveErr
			}
			return refreshResp.AccessToken, nil
		}

		// Refresh rejected (400/401): fall back to full OAuth flow
		if !errors.Is(refreshErr, token.ErrRefreshRejected) {
			return "", refreshErr
		}
	}

	// Full OAuth flow (first time, corrupted token, or refresh rejected)
	return performFullOAuthFlow(cfg, storage)
}

// performFullOAuthFlow runs the complete OAuth2.1 authorization code flow with PKCE.
func performFullOAuthFlow(cfg *config.Config, storage *token.Storage) (string, error) {
	discovery, err := oauth.DiscoverEndpoints(cfg.ServerURL)
	if err != nil {
		return "", err
	}

	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		return "", apperrors.NewAuthError("failed to generate PKCE codes", err)
	}

	callbackServer, err := oauth.NewCallbackServer()
	if err != nil {
		return "", err
	}

	redirectURI, err := callbackServer.Start()
	if err != nil {
		return "", err
	}
	defer callbackServer.Stop()

	authURL := buildAuthorizationURL(
		discovery.AuthorizationEndpoint,
		cfg.ClientID,
		redirectURI,
		pkce.CodeChallenge,
	)

	fmt.Fprintf(os.Stderr, "Opening browser for authorization...\n")
	if err := oauth.OpenBrowser(authURL); err != nil {
		return "", err
	}

	fmt.Fprintf(os.Stderr, "Waiting for authorization callback...\n")
	code, err := callbackServer.WaitForCallback(5 * time.Minute)
	if err != nil {
		return "", err
	}

	fmt.Fprintf(os.Stderr, "Exchanging authorization code for tokens...\n")
	tokenResp, err := oauth.ExchangeCodeForToken(
		discovery.TokenEndpoint,
		cfg.ClientID,
		cfg.ClientSecret,
		code,
		redirectURI,
		pkce.CodeVerifier,
	)
	if err != nil {
		return "", err
	}

	if err := storage.Save(cfg.ServerURL, tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}

// buildAuthorizationURL constructs the OAuth authorization URL with PKCE.
func buildAuthorizationURL(authEndpoint, clientID, redirectURI, codeChallenge string) string {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	return fmt.Sprintf("%s?%s", authEndpoint, params.Encode())
}
