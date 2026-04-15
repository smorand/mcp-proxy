package oauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/smorand/mcp-proxy/internal/errors"
)

// CallbackResult holds the result of the OAuth callback
type CallbackResult struct {
	Code  string
	Error string
}

// CallbackServer manages the temporary HTTP server for OAuth callbacks
type CallbackServer struct {
	server   *http.Server
	port     int
	resultCh chan CallbackResult
}

// NewCallbackServer creates a new callback server
// It tries ports 3000-3010 until it finds an available one
func NewCallbackServer() (*CallbackServer, error) {
	// Try ports 3000-3010
	for port := 3000; port <= 3010; port++ {
		// Try to listen on the port to verify it's available
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			continue // Port in use, try next
		}
		// Close immediately, we'll reopen when starting
		listener.Close()

		return &CallbackServer{
			port:     port,
			resultCh: make(chan CallbackResult, 1),
		}, nil
	}

	return nil, errors.NewNetworkError("no available ports in range 3000-3010", nil)
}

// Start starts the HTTP server and returns the redirect URI
func (s *CallbackServer) Start() (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", s.handleCallback)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in background
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.resultCh <- CallbackResult{Error: fmt.Sprintf("HTTP server error: %v", err)}
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	return fmt.Sprintf("http://localhost:%d/oauth2callback", s.port), nil
}

// handleCallback handles the OAuth callback request
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Extract authorization code or error from query parameters
	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		if errorDesc != "" {
			errorParam = fmt.Sprintf("%s: %s", errorParam, errorDesc)
		}
		s.resultCh <- CallbackResult{Error: errorParam}
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "OAuth error: %s", errorParam)
		return
	}

	if code == "" {
		s.resultCh <- CallbackResult{Error: "no authorization code received"}
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Error: No authorization code received")
		return
	}

	// Send success response
	s.resultCh <- CallbackResult{Code: code}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Authorization successful! You can close this window.")
}

// WaitForCallback waits for the OAuth callback with a timeout
func (s *CallbackServer) WaitForCallback(timeout time.Duration) (string, error) {
	select {
	case result := <-s.resultCh:
		if result.Error != "" {
			return "", errors.NewAuthError(fmt.Sprintf("OAuth callback error: %s", result.Error), nil)
		}
		return result.Code, nil
	case <-time.After(timeout):
		return "", errors.NewAuthError("OAuth flow timed out. User did not complete authentication.", nil)
	}
}

// Stop stops the HTTP server
func (s *CallbackServer) Stop() error {
	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// Port returns the port the server is listening on
func (s *CallbackServer) Port() int {
	return s.port
}
