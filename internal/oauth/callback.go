package oauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/smorand/mcp-proxy/internal/apperr"
)

const (
	callbackPortStart    = 3000
	callbackPortEnd      = 3010
	callbackReadTimeout  = 10 * time.Second
	callbackWriteTimeout = 10 * time.Second
	callbackStartDelay   = 100 * time.Millisecond
	callbackShutdown     = 5 * time.Second
)

// CallbackResult holds either the authorization code or the error returned
// by the OAuth provider on the redirect URI.
type CallbackResult struct {
	Code  string
	Error string
}

// CallbackServer is a single-shot localhost HTTP server that captures the
// OAuth authorization code via the redirect URI.
type CallbackServer struct {
	server   *http.Server
	port     int
	resultCh chan CallbackResult
}

// NewCallbackServer reserves an available local port in the configured
// range and returns a server ready to be started.
func NewCallbackServer() (*CallbackServer, error) {
	for port := callbackPortStart; port <= callbackPortEnd; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		_ = listener.Close()
		return &CallbackServer{
			port:     port,
			resultCh: make(chan CallbackResult, 1),
		}, nil
	}
	return nil, apperr.NewNetworkError(
		fmt.Sprintf("no available ports in range %d-%d", callbackPortStart, callbackPortEnd),
		nil,
	)
}

// Start launches the HTTP server in the background and returns the
// redirect URI to use in the authorization request.
func (s *CallbackServer) Start() (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", s.handleCallback)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:      mux,
		ReadTimeout:  callbackReadTimeout,
		WriteTimeout: callbackWriteTimeout,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.resultCh <- CallbackResult{Error: fmt.Sprintf("HTTP server error: %v", err)}
		}
	}()

	time.Sleep(callbackStartDelay)
	return fmt.Sprintf("http://localhost:%d/oauth2callback", s.port), nil
}

// WaitForCallback waits for the OAuth callback or the given timeout.
func (s *CallbackServer) WaitForCallback(timeout time.Duration) (string, error) {
	select {
	case result := <-s.resultCh:
		if result.Error != "" {
			return "", apperr.NewAuthError(fmt.Sprintf("OAuth callback error: %s", result.Error), nil)
		}
		return result.Code, nil
	case <-time.After(timeout):
		return "", apperr.NewAuthError("OAuth flow timed out. User did not complete authentication.", nil)
	}
}

// Stop gracefully shuts down the HTTP server.
func (s *CallbackServer) Stop() error {
	if s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), callbackShutdown)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// Port returns the bound TCP port.
func (s *CallbackServer) Port() int {
	return s.port
}

func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		if errorDesc != "" {
			errorParam = fmt.Sprintf("%s: %s", errorParam, errorDesc)
		}
		s.resultCh <- CallbackResult{Error: errorParam}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		// errorParam is echoed back as plain text only; no HTML interpretation.
		_, _ = w.Write([]byte("OAuth error: " + errorParam)) // #nosec G705 -- Content-Type forced to text/plain above.
		return
	}

	if code == "" {
		s.resultCh <- CallbackResult{Error: "no authorization code received"}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, "Error: No authorization code received")
		return
	}

	s.resultCh <- CallbackResult{Code: code}
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "Authorization successful! You can close this window.")
}
