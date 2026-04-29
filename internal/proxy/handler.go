package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"

	"github.com/smorand/mcp-proxy/internal/apperr"
	"github.com/smorand/mcp-proxy/internal/oauth"
	"github.com/smorand/mcp-proxy/internal/telemetry"
	"github.com/smorand/mcp-proxy/internal/token"
)

// HandlerConfig contains configuration for creating a Handler.
type HandlerConfig struct {
	AccessToken  string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
	ServerURL    string
	Stdin        io.Reader
	Stdout       io.Writer
	Storage      *token.Storage
	Tracer       *telemetry.Tracer
}

// Handler orchestrates the MCP proxy: reads JSON-RPC from stdin,
// forwards to MCP server via HTTP, writes responses to stdout.
type Handler struct {
	accessToken  string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	mcpClient    *MCPClient
	serverURL    string
	stdin        io.Reader
	stdout       io.Writer
	storage      *token.Storage
	tracer       *telemetry.Tracer
}

// NewHandler creates a new proxy handler.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		accessToken:  cfg.AccessToken,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		httpClient:   cfg.HTTPClient,
		mcpClient:    NewMCPClient(cfg.ServerURL, cfg.HTTPClient),
		serverURL:    cfg.ServerURL,
		stdin:        cfg.Stdin,
		stdout:       cfg.Stdout,
		storage:      cfg.Storage,
		tracer:       cfg.Tracer,
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

func (h *Handler) handleMessage(ctx context.Context, message []byte) error {
	ctx, span := h.startSpan(ctx, "http.forward",
		telemetry.StringAttr("http.method", http.MethodPost),
		telemetry.StringAttr("mcp.server.url", h.serverURL),
	)

	result, err := h.forwardWithRetry(ctx, message, span)
	if err != nil {
		h.endSpan(span, err)
		return err
	}

	if result.StatusCode == http.StatusAccepted {
		_ = result.Body.Close()
		h.endSpan(span, nil)
		return nil
	}

	if err := h.writeResponse(result); err != nil {
		h.endSpan(span, err)
		return err
	}

	h.endSpan(span, nil)
	return nil
}

func (h *Handler) forwardWithRetry(ctx context.Context, message []byte, span *telemetry.SpanContext) (*ForwardResult, error) {
	result, err := h.mcpClient.Forward(ctx, message, h.accessToken)
	if err != nil {
		return nil, err
	}
	if span != nil {
		span.SetAttribute("http.status_code", result.StatusCode)
	}

	if result.StatusCode != http.StatusUnauthorized {
		return result, nil
	}

	_ = result.Body.Close()
	if span != nil {
		span.SetAttribute("token.refresh_attempted", true)
	}

	if err := h.refreshToken(ctx); err != nil {
		return nil, err
	}

	result, err = h.mcpClient.Forward(ctx, message, h.accessToken)
	if err != nil {
		return nil, err
	}
	if span != nil {
		span.SetAttribute("http.status_code", result.StatusCode)
	}
	return result, nil
}

func (h *Handler) writeResponse(result *ForwardResult) error {
	if strings.Contains(result.ContentType, "text/event-stream") {
		if err := WriteSSEDataTo(h.stdout, result.Body); err != nil {
			return apperr.NewNetworkError("failed to stream MCP response", err)
		}
		return nil
	}

	defer func() { _ = result.Body.Close() }()
	if _, err := io.Copy(h.stdout, result.Body); err != nil {
		return apperr.NewNetworkError("failed to stream MCP response", err)
	}
	if _, err := h.stdout.Write([]byte("\n")); err != nil {
		return apperr.NewNetworkError("failed to write response separator", err)
	}
	return nil
}

func (h *Handler) refreshToken(ctx context.Context) error {
	ctx, span := h.startSpan(ctx, "oauth.token_refresh",
		telemetry.StringAttr("oauth.flow.step", "refresh"),
		telemetry.StringAttr("mcp.server.url", h.serverURL),
	)

	tokenData, err := h.storage.Load(h.serverURL)
	if err != nil || !tokenData.HasRefreshToken() {
		refreshErr := apperr.NewTokenError("token rejected and no refresh token available", nil)
		h.endSpan(span, refreshErr)
		return refreshErr
	}

	discovery, err := oauth.DiscoverEndpoints(ctx, h.serverURL, h.httpClient)
	if err != nil {
		h.endSpan(span, err)
		return err
	}

	refreshResp, err := token.RefreshAccessToken(
		ctx,
		h.httpClient,
		discovery.TokenEndpoint,
		h.clientID,
		h.clientSecret,
		tokenData.RefreshToken,
	)
	if err != nil {
		tokenErr := apperr.NewTokenError(fmt.Sprintf("token refresh failed: %v", err), err)
		h.endSpan(span, tokenErr)
		return tokenErr
	}

	newRefresh := refreshResp.RefreshToken
	if newRefresh == "" {
		newRefresh = tokenData.RefreshToken
	}
	if err := h.storage.Save(h.serverURL, refreshResp.AccessToken, newRefresh, refreshResp.ExpiresIn); err != nil {
		h.endSpan(span, err)
		return err
	}

	h.accessToken = refreshResp.AccessToken
	if span != nil {
		span.SetAttribute("token.refresh_attempted", true)
	}
	h.endSpan(span, nil)
	return nil
}

func (h *Handler) startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, *telemetry.SpanContext) {
	if h.tracer == nil {
		return ctx, nil
	}
	return h.tracer.StartSpan(ctx, name, attrs...)
}

func (h *Handler) endSpan(span *telemetry.SpanContext, err error) {
	if span == nil {
		return
	}
	span.End(err)
}
