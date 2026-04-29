// Package app wires the components of the mcp-proxy CLI together.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/smorand/mcp-proxy/internal/apperr"
	"github.com/smorand/mcp-proxy/internal/config"
	"github.com/smorand/mcp-proxy/internal/oauth"
	"github.com/smorand/mcp-proxy/internal/proxy"
	"github.com/smorand/mcp-proxy/internal/telemetry"
	"github.com/smorand/mcp-proxy/internal/token"
)

const (
	traceFileName       = "traces.jsonl"
	cacheDirPerm        = 0700
	httpClientTimeout   = 10 * time.Second
	mcpHTTPTimeout      = 30 * time.Second
	oauthCallbackWait   = 5 * time.Minute
	authCodeChallengeMD = "S256"
	serviceName         = "mcp-proxy"
)

// Run is the entry point of the CLI. It returns an error suitable for
// apperr.Fatal.
func Run() error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Parse()
	if err != nil {
		return err
	}

	storage, err := token.NewStorage()
	if err != nil {
		return err
	}

	tracer, err := initTracer(storage.CacheDir())
	if err != nil {
		return err
	}
	if tracer != nil {
		defer func() {
			if cerr := tracer.Close(); cerr != nil {
				slog.Warn("tracer shutdown failed", "err", cerr)
			}
		}()
	}

	httpClient := &http.Client{Timeout: httpClientTimeout}
	mcpClient := &http.Client{Timeout: mcpHTTPTimeout}

	ctx, cancel := signalContext()
	defer cancel()

	accessToken, err := acquireToken(ctx, cfg, storage, tracer, httpClient)
	if err != nil {
		return err
	}

	handler := proxy.NewHandler(proxy.HandlerConfig{
		AccessToken:  accessToken,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		HTTPClient:   mcpClient,
		ServerURL:    cfg.ServerURL,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Storage:      storage,
		Tracer:       tracer,
	})
	return handler.Run(ctx)
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	return ctx, cancel
}

func initTracer(cacheDir string) (*telemetry.Tracer, error) {
	if err := os.MkdirAll(cacheDir, cacheDirPerm); err != nil {
		return nil, apperr.NewFileSystemError(
			fmt.Sprintf("Cannot create cache directory at %s", cacheDir),
			err,
		)
	}
	tracePath := filepath.Join(cacheDir, traceFileName)
	tracer, err := telemetry.NewTracer(tracePath, serviceName)
	if err != nil {
		return nil, apperr.NewFileSystemError("failed to initialize telemetry", err)
	}
	return tracer, nil
}

func acquireToken(
	ctx context.Context,
	cfg *config.Config,
	storage *token.Storage,
	tracer *telemetry.Tracer,
	client *http.Client,
) (string, error) {
	var span *telemetry.SpanContext
	if tracer != nil {
		_, span = tracer.StartSpan(ctx, "oauth.token_check",
			telemetry.StringAttr("oauth.flow.step", "token_validation"),
			telemetry.StringAttr("mcp.server.url", cfg.ServerURL),
		)
		defer span.End(nil)
	}

	tokenData, loadErr := storage.Load(cfg.ServerURL)
	if loadErr == nil && !tokenData.IsExpired(time.Now()) {
		return tokenData.AccessToken, nil
	}

	if loadErr == nil && tokenData.IsExpired(time.Now()) && tokenData.HasRefreshToken() {
		accessToken, refreshErr := refreshToken(ctx, cfg, storage, tokenData, client)
		if refreshErr == nil {
			return accessToken, nil
		}
		if !errors.Is(refreshErr, token.ErrRefreshRejected) {
			return "", refreshErr
		}
	}

	return performOAuthFlow(ctx, cfg, storage, client)
}

func refreshToken(
	ctx context.Context,
	cfg *config.Config,
	storage *token.Storage,
	tokenData *token.TokenData,
	client *http.Client,
) (string, error) {
	discovery, err := oauth.DiscoverEndpoints(ctx, cfg.ServerURL, client)
	if err != nil {
		return "", err
	}

	resp, err := token.RefreshAccessToken(
		ctx,
		client,
		discovery.TokenEndpoint,
		cfg.ClientID,
		cfg.ClientSecret,
		tokenData.RefreshToken,
	)
	if err != nil {
		return "", err
	}

	newRefresh := resp.RefreshToken
	if newRefresh == "" {
		newRefresh = tokenData.RefreshToken
	}
	if err := storage.Save(cfg.ServerURL, resp.AccessToken, newRefresh, resp.ExpiresIn); err != nil {
		return "", err
	}
	return resp.AccessToken, nil
}

func performOAuthFlow(
	ctx context.Context,
	cfg *config.Config,
	storage *token.Storage,
	client *http.Client,
) (string, error) {
	discovery, err := oauth.DiscoverEndpoints(ctx, cfg.ServerURL, client)
	if err != nil {
		return "", err
	}

	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		return "", apperr.NewAuthError("failed to generate PKCE codes", err)
	}

	callbackServer, err := oauth.NewCallbackServer()
	if err != nil {
		return "", err
	}
	redirectURI, err := callbackServer.Start()
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := callbackServer.Stop(); cerr != nil {
			slog.Warn("callback server shutdown failed", "err", cerr)
		}
	}()

	authURL := buildAuthorizationURL(
		discovery.AuthorizationEndpoint,
		cfg.ClientID,
		redirectURI,
		pkce.CodeChallenge,
	)

	slog.Info("opening browser for authorization")
	if err := oauth.OpenBrowser(authURL); err != nil {
		return "", err
	}

	slog.Info("waiting for authorization callback")
	code, err := callbackServer.WaitForCallback(oauthCallbackWait)
	if err != nil {
		return "", err
	}

	slog.Info("exchanging authorization code for tokens")
	tokenResp, err := oauth.ExchangeCodeForToken(
		ctx,
		client,
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

func buildAuthorizationURL(authEndpoint, clientID, redirectURI, codeChallenge string) string {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", authCodeChallengeMD)
	return fmt.Sprintf("%s?%s", authEndpoint, params.Encode())
}
