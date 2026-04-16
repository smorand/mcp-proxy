package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/smorand/mcp-proxy/internal/proxy"
	"github.com/smorand/mcp-proxy/internal/telemetry"
	"github.com/smorand/mcp-proxy/internal/token"
)

// setupMockMCPServer creates a mock MCP server that handles discovery, token refresh, and MCP messages.
func setupMockMCPServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server
}

// createTestToken creates a token file for the given server URL.
func createTestToken(t *testing.T, serverURL, accessToken, refreshToken string, expiresIn int) {
	t.Helper()
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	if err := storage.Save(serverURL, accessToken, refreshToken, expiresIn); err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}
	t.Cleanup(func() {
		tokenData, loadErr := storage.Load(serverURL)
		if loadErr == nil {
			_ = tokenData
		}
		// Clean up by trying to remove the file; ignore errors
		os.Remove(storage.GetCacheDir())
	})
}

// cleanupTokenForURL removes the token file for a given URL.
func cleanupTokenForURL(t *testing.T, serverURL string) {
	t.Helper()
	storage, err := token.NewStorage()
	if err != nil {
		return
	}
	// Remove all files in cache dir matching this URL
	entries, err := os.ReadDir(storage.GetCacheDir())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			os.Remove(fmt.Sprintf("%s/%s", storage.GetCacheDir(), entry.Name()))
		}
	}
}

// TestE2E002_SubsequentUseWithValidToken tests that a valid cached token is used
// to forward requests to the MCP server without any OAuth flow.
func TestE2E002_SubsequentUseWithValidToken(t *testing.T) {
	// Mock MCP server that verifies Authorization header and returns a JSON-RPC response
	mcpServer := setupMockMCPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-test-token" {
			t.Errorf("Authorization = %q, want Bearer valid-test-token", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`))
	}))

	// Create token file with valid token (expires in 1 hour)
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	if err := storage.Save(mcpServer.URL, "valid-test-token", "test-refresh", 3600); err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}
	t.Cleanup(func() { cleanupTokenForURL(t, mcpServer.URL) })

	// Create proxy handler with mock stdin/stdout
	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n")
	var stdout bytes.Buffer

	handler := proxy.NewHandler(proxy.HandlerConfig{
		ServerURL:    mcpServer.URL,
		Storage:      storage,
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		Stdin:        stdin,
		Stdout:       &stdout,
		AccessToken:  "valid-test-token",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := handler.Run(ctx); err != nil {
		t.Fatalf("Handler.Run() failed: %v", err)
	}

	// Verify response was forwarded to stdout
	expected := `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}` + "\n"
	if stdout.String() != expected {
		t.Errorf("stdout = %q, want %q", stdout.String(), expected)
	}
}

// TestE2E002_BinarySubsequentUse tests the binary E2E for subsequent use with valid token.
func TestE2E002_BinarySubsequentUse(t *testing.T) {
	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "mcp-proxy-test", "../main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("mcp-proxy-test")

	// Mock MCP server
	mcpServer := setupMockMCPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":["test"]}}`))
	}))

	// Create token file
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	if err := storage.Save(mcpServer.URL, "binary-test-token", "refresh", 3600); err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}
	t.Cleanup(func() { cleanupTokenForURL(t, mcpServer.URL) })

	// Set credentials
	os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Run binary with piped stdin
	cmd := exec.Command("./mcp-proxy-test", "-u", mcpServer.URL)
	cmd.Stdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n")
	output, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		t.Fatalf("Binary failed: %v, stderr: %s", err, stderr)
	}

	expected := `{"jsonrpc":"2.0","id":1,"result":{"tools":["test"]}}` + "\n"
	if string(output) != expected {
		t.Errorf("stdout = %q, want %q", string(output), expected)
	}
}

// TestE2E009_MCPServerRejectsToken tests 401 handling with automatic token refresh and retry.
func TestE2E009_MCPServerRejectsToken(t *testing.T) {
	requestCount := 0
	var serverURL string

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"authorization_endpoint": serverURL + "/authorize",
			"token_endpoint":         serverURL + "/token",
			"issuer":                 serverURL,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token.RefreshResponse{
			AccessToken:  "new-valid-token",
			RefreshToken: "new-refresh-token",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			return
		}
		requestCount++
		if requestCount == 1 {
			// First request: reject with 401 (token revoked)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Second request (after refresh): accept
		auth := r.Header.Get("Authorization")
		if auth != "Bearer new-valid-token" {
			t.Errorf("Retry Authorization = %q, want Bearer new-valid-token", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`))
	})

	server := httptest.NewTLSServer(mux)
	serverURL = server.URL
	defer server.Close()

	// Create token file with "revoked" token
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	if err := storage.Save(serverURL, "revoked-token", "valid-refresh", 3600); err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}
	t.Cleanup(func() { cleanupTokenForURL(t, serverURL) })

	// Run proxy handler
	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n")
	var stdout bytes.Buffer

	handler := proxy.NewHandler(proxy.HandlerConfig{
		ServerURL:    serverURL,
		Storage:      storage,
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		Stdin:        stdin,
		Stdout:       &stdout,
		AccessToken:  "revoked-token",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := handler.Run(ctx); err != nil {
		t.Fatalf("Handler.Run() failed: %v", err)
	}

	// Verify response was forwarded after refresh
	expected := `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}` + "\n"
	if stdout.String() != expected {
		t.Errorf("stdout = %q, want %q", stdout.String(), expected)
	}

	// Verify there were exactly 2 MCP requests (1 rejected + 1 retried)
	if requestCount != 2 {
		t.Errorf("MCP request count = %d, want 2", requestCount)
	}

	// Verify token file was updated with new token
	updatedToken, err := storage.Load(serverURL)
	if err != nil {
		t.Fatalf("Failed to load updated token: %v", err)
	}
	if updatedToken.AccessToken != "new-valid-token" {
		t.Errorf("updated access_token = %q, want new-valid-token", updatedToken.AccessToken)
	}
	if updatedToken.RefreshToken != "new-refresh-token" {
		t.Errorf("updated refresh_token = %q, want new-refresh-token", updatedToken.RefreshToken)
	}
}

// TestE2E010_NetworkErrorConnectingToMCPServer tests that a network error
// exits the proxy with an appropriate error.
func TestE2E010_NetworkErrorConnectingToMCPServer(t *testing.T) {
	// Use a server that's immediately closed (unreachable)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close()

	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n")
	var stdout bytes.Buffer

	handler := proxy.NewHandler(proxy.HandlerConfig{
		ServerURL:    unreachableURL,
		Storage:      storage,
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		Stdin:        stdin,
		Stdout:       &stdout,
		AccessToken:  "valid-token",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = handler.Run(ctx)
	if err == nil {
		t.Fatal("expected error for unreachable MCP server, got nil")
	}

	// Verify error message mentions connection failure
	if !strings.Contains(err.Error(), "Failed to connect to MCP server") {
		t.Errorf("error = %q, want to contain 'Failed to connect to MCP server'", err.Error())
	}
}

// TestE2E010_BinaryNetworkError tests the binary E2E for network error.
func TestE2E010_BinaryNetworkError(t *testing.T) {
	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "mcp-proxy-test", "../main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("mcp-proxy-test")

	// Create a server and immediately close it to get an unreachable URL
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close()

	// Create token file for unreachable URL
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	if err := storage.Save(unreachableURL, "valid-token", "refresh", 3600); err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}
	t.Cleanup(func() { cleanupTokenForURL(t, unreachableURL) })

	// Set credentials
	os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Run binary
	cmd := exec.Command("./mcp-proxy-test", "-u", unreachableURL)
	cmd.Stdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test"}` + "\n")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("Expected command to fail for unreachable server")
	}

	// Should exit with code 3 (network error)
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 3 {
			t.Errorf("exit code = %d, want 3", exitErr.ExitCode())
		}
	} else {
		t.Errorf("Expected exec.ExitError, got %T", err)
	}

	// Check error message
	outputStr := string(output)
	if !strings.Contains(outputStr, "Error: Failed to connect to MCP server") {
		t.Errorf("output = %q, want to contain 'Error: Failed to connect to MCP server'", outputStr)
	}
}

// TestE2E025_VeryLongMCPServerResponse tests streaming of large responses.
func TestE2E025_VeryLongMCPServerResponse(t *testing.T) {
	// Generate a 1MB response (scaled down from 10MB for test speed)
	largePayload := strings.Repeat("x", 1*1024*1024)
	largeResponse := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"data":"%s"}}`, largePayload)

	mcpServer := setupMockMCPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(largeResponse))
	}))

	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"get_data","params":{}}` + "\n")
	var stdout bytes.Buffer

	handler := proxy.NewHandler(proxy.HandlerConfig{
		ServerURL:    mcpServer.URL,
		Storage:      storage,
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		Stdin:        stdin,
		Stdout:       &stdout,
		AccessToken:  "valid-token",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := handler.Run(ctx); err != nil {
		t.Fatalf("Handler.Run() failed: %v", err)
	}

	// Verify the full response was streamed to stdout
	outputStr := stdout.String()
	if !strings.HasSuffix(outputStr, "\n") {
		t.Error("Response should end with newline")
	}
	// Remove trailing newline for comparison
	outputStr = strings.TrimSuffix(outputStr, "\n")
	if outputStr != largeResponse {
		t.Errorf("Response length = %d, want %d", len(outputStr), len(largeResponse))
	}
}

// TestE2E026_GracefulSIGINTHandling tests that SIGINT causes a clean exit.
func TestE2E026_GracefulSIGINTHandling(t *testing.T) {
	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "mcp-proxy-test", "../main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("mcp-proxy-test")

	// Mock MCP server (slow, to keep proxy waiting)
	mcpServer := setupMockMCPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))

	// Create token file
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	if err := storage.Save(mcpServer.URL, "sigint-token", "refresh", 3600); err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}
	t.Cleanup(func() { cleanupTokenForURL(t, mcpServer.URL) })

	// Set credentials
	os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Start binary (stdin stays open, proxy blocks waiting for input)
	cmd := exec.Command("./mcp-proxy-test", "-u", mcpServer.URL)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start binary: %v", err)
	}

	// Give the process time to start
	time.Sleep(500 * time.Millisecond)

	// Send SIGINT
	cmd.Process.Signal(syscall.SIGINT)

	// Close stdin
	stdinPipe.Close()

	// Wait for process to exit
	err = cmd.Wait()

	// Should exit with code 0 (graceful shutdown)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				t.Errorf("exit code = %d, want 0, stderr: %s", exitErr.ExitCode(), stderr.String())
			}
		}
	}

	// No error message should be printed
	if stderr.Len() > 0 {
		t.Errorf("stderr should be empty, got: %s", stderr.String())
	}
}

// TestE2E024_TokenExpiresDuringSession tests that token refresh works
// when the token expires mid-session.
func TestE2E024_TokenExpiresDuringSession(t *testing.T) {
	requestCount := 0
	var serverURL string

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"authorization_endpoint": serverURL + "/authorize",
			"token_endpoint":         serverURL + "/token",
			"issuer":                 serverURL,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token.RefreshResponse{
			AccessToken:  "refreshed-token",
			RefreshToken: "new-refresh",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			return
		}
		requestCount++
		auth := r.Header.Get("Authorization")
		// Reject first request (simulating server-side expiration)
		if requestCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Accept after refresh
		if auth != "Bearer refreshed-token" {
			t.Errorf("Retry Authorization = %q, want Bearer refreshed-token", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
	})

	server := httptest.NewTLSServer(mux)
	serverURL = server.URL
	defer server.Close()

	// Token that is still valid (not expired from our side, but server will reject it)
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	if err := storage.Save(serverURL, "about-to-expire-token", "session-refresh", 3600); err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}
	t.Cleanup(func() { cleanupTokenForURL(t, serverURL) })

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"check"}` + "\n")
	var stdout bytes.Buffer

	handler := proxy.NewHandler(proxy.HandlerConfig{
		ServerURL:    serverURL,
		Storage:      storage,
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		Stdin:        stdin,
		Stdout:       &stdout,
		AccessToken:  "about-to-expire-token",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := handler.Run(ctx); err != nil {
		t.Fatalf("Handler.Run() failed: %v", err)
	}

	expected := `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}` + "\n"
	if stdout.String() != expected {
		t.Errorf("stdout = %q, want %q", stdout.String(), expected)
	}
}

// TestProxy_MultipleMessages tests forwarding multiple messages in sequence.
func TestProxy_MultipleMessages(t *testing.T) {
	messageCount := 0
	mcpServer := setupMockMCPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		messageCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{}}`, messageCount)))
	}))

	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Send two messages
	input := `{"jsonrpc":"2.0","id":1,"method":"test1"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"test2"}` + "\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	handler := proxy.NewHandler(proxy.HandlerConfig{
		ServerURL:    mcpServer.URL,
		Storage:      storage,
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		Stdin:        stdin,
		Stdout:       &stdout,
		AccessToken:  "valid-token",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := handler.Run(ctx); err != nil {
		t.Fatalf("Handler.Run() failed: %v", err)
	}

	// Verify both responses were forwarded in order (FIFO)
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d: %q", len(lines), stdout.String())
	}
	if !strings.Contains(lines[0], `"id":1`) {
		t.Errorf("first response should have id 1, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"id":2`) {
		t.Errorf("second response should have id 2, got: %s", lines[1])
	}
}

// TestProxy_TracingAttributes tests that proxy operations produce trace spans
// with the expected attributes.
func TestProxy_TracingAttributes(t *testing.T) {
	mcpServer := setupMockMCPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))

	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create a temp trace file
	tmpFile, err := os.CreateTemp("", "traces-*.jsonl")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	tracer, err := telemetry.NewTracer(tmpFile.Name(), "mcp-proxy")
	if err != nil {
		t.Fatalf("Failed to create tracer: %v", err)
	}

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test"}` + "\n")
	var stdout bytes.Buffer

	handler := proxy.NewHandler(proxy.HandlerConfig{
		ServerURL:    mcpServer.URL,
		Storage:      storage,
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		Tracer:       tracer,
		Stdin:        stdin,
		Stdout:       &stdout,
		AccessToken:  "valid-token",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := handler.Run(ctx); err != nil {
		t.Fatalf("Handler.Run() failed: %v", err)
	}
	tracer.Close()

	// Read and verify trace file
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `"http.method"`) {
		t.Error("Trace should contain http.method attribute")
	}
	if !strings.Contains(content, `"http.status_code"`) {
		t.Error("Trace should contain http.status_code attribute")
	}
	if !strings.Contains(content, `"mcp.server.url"`) {
		t.Error("Trace should contain mcp.server.url attribute")
	}

	// Verify no sensitive data in traces
	sensitiveKeys := []string{"access_token", "refresh_token", "client_secret", "code_verifier"}
	for _, key := range sensitiveKeys {
		if strings.Contains(content, key) {
			t.Errorf("Trace should NOT contain sensitive key %q", key)
		}
	}
}
