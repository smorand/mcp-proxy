package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/smorand/mcp-proxy/internal/errors"
	"github.com/smorand/mcp-proxy/internal/oauth"
	"github.com/smorand/mcp-proxy/internal/telemetry"
	"github.com/smorand/mcp-proxy/internal/token"
)

// HandlerConfig contains configuration for creating a Handler.
type HandlerConfig struct {
	ServerURL    string
	Storage      *token.Storage
	ClientID     string
	ClientSecret string
	Tracer       *telemetry.Tracer
	Stdin        io.Reader
	Stdout       io.Writer
	AccessToken  string
}

// Handler orchestrates the MCP proxy: reads JSON-RPC from stdin,
// forwards to MCP server via HTTP, writes responses to stdout.
type Handler struct {
	serverURL    string
	mcpClient    *MCPClient
	storage      *token.Storage
	clientID     string
	clientSecret string
	tracer       *telemetry.Tracer
	stdin        io.Reader
	stdout       io.Writer
	accessToken  string
}

// NewHandler creates a new proxy handler.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		serverURL:    cfg.ServerURL,
		mcpClient:    NewMCPClient(cfg.ServerURL),
		storage:      cfg.Storage,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		tracer:       cfg.Tracer,
		stdin:        cfg.Stdin,
		stdout:       cfg.Stdout,
		accessToken:  cfg.AccessToken,
	}
}

// Run starts the proxy loop. It reads JSON-RPC messages from stdin,
// forwards them to the MCP server, and writes responses to stdout.
// It stops when stdin is closed or the context is canceled.
func (h *Handler) Run(ctx context.Context) error {
	messages := make(chan []byte)
	scanErr := make(chan error, 1)

	go func() {
		scanner := NewScanner(h.stdin)
		for scanner.Scan() {
			raw := scanner.Bytes()
			if len(raw) == 0 {
				continue
			}
			msg := make([]byte, len(raw))
			copy(msg, raw)
			select {
			case messages <- msg:
			case <-ctx.Done():
				return
			}
		}
		scanErr <- scanner.Err()
		close(messages)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-messages:
			if !ok {
				select {
				case err := <-scanErr:
					return err
				default:
					return nil
				}
			}
			if err := h.handleMessage(ctx, msg); err != nil {
				return err
			}
		}
	}
}

// handleMessage forwards a single JSON-RPC message to the MCP server.
func (h *Handler) handleMessage(ctx context.Context, message []byte) error {
	var span *telemetry.SpanContext
	if h.tracer != nil {
		_, span = h.tracer.StartSpan(ctx, "http.forward",
			telemetry.StringAttr("http.method", "POST"),
			telemetry.StringAttr("mcp.server.url", h.serverURL),
		)
	}

	result, err := h.mcpClient.Forward(ctx, message, h.accessToken)
	if err != nil {
		if span != nil {
			span.End(err)
		}
		return err
	}

	if span != nil {
		span.SetAttribute("http.status_code", result.StatusCode)
	}

	// Handle 401 Unauthorized: refresh token and retry once
	if result.StatusCode == http.StatusUnauthorized {
		result.Body.Close()

		if span != nil {
			span.SetAttribute("token.refresh_attempted", true)
		}

		if err := h.refreshToken(ctx); err != nil {
			if span != nil {
				span.End(err)
			}
			return err
		}

		// Retry with refreshed token
		result, err = h.mcpClient.Forward(ctx, message, h.accessToken)
		if err != nil {
			if span != nil {
				span.End(err)
			}
			return err
		}

		if span != nil {
			span.SetAttribute("http.status_code", result.StatusCode)
		}
	}

	defer result.Body.Close()

	// Stream response to stdout without buffering entire body
	if _, err := io.Copy(h.stdout, result.Body); err != nil {
		if span != nil {
			span.End(err)
		}
		return errors.NewNetworkError("failed to stream MCP response", err)
	}

	// Newline separator for JSON-RPC over stdio
	if _, err := h.stdout.Write([]byte("\n")); err != nil {
		if span != nil {
			span.End(err)
		}
		return errors.NewNetworkError("failed to write response separator", err)
	}

	if span != nil {
		span.End(nil)
	}

	return nil
}

// refreshToken attempts to refresh the access token using the refresh_token.
func (h *Handler) refreshToken(ctx context.Context) error {
	var span *telemetry.SpanContext
	if h.tracer != nil {
		_, span = h.tracer.StartSpan(ctx, "oauth.token_refresh",
			telemetry.StringAttr("oauth.flow.step", "refresh"),
			telemetry.StringAttr("mcp.server.url", h.serverURL),
		)
	}

	tokenData, err := h.storage.Load(h.serverURL)
	if err != nil || !tokenData.HasRefreshToken() {
		refreshErr := errors.NewTokenError("token rejected and no refresh token available", nil)
		if span != nil {
			span.End(refreshErr)
		}
		return refreshErr
	}

	discovery, err := oauth.DiscoverEndpoints(h.serverURL)
	if err != nil {
		if span != nil {
			span.End(err)
		}
		return err
	}

	refreshResp, err := token.RefreshAccessToken(
		discovery.TokenEndpoint,
		h.clientID,
		h.clientSecret,
		tokenData.RefreshToken,
	)
	if err != nil {
		tokenErr := errors.NewTokenError(
			fmt.Sprintf("token refresh failed: %v", err),
			err,
		)
		if span != nil {
			span.End(tokenErr)
		}
		return tokenErr
	}

	newRefresh := refreshResp.RefreshToken
	if newRefresh == "" {
		newRefresh = tokenData.RefreshToken
	}
	if err := h.storage.Save(h.serverURL, refreshResp.AccessToken, newRefresh, refreshResp.ExpiresIn); err != nil {
		if span != nil {
			span.End(err)
		}
		return err
	}

	h.accessToken = refreshResp.AccessToken

	if span != nil {
		span.SetAttribute("token.refresh_attempted", true)
		span.End(nil)
	}

	return nil
}
