package tests

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/smorand/mcp-proxy/internal/oauth"
)

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestCallbackServer(t *testing.T) {
	t.Run("E2E-019: Temporary HTTP server lifecycle", func(t *testing.T) {
		// Create callback server
		server, err := oauth.NewCallbackServer()
		if err != nil {
			t.Fatalf("NewCallbackServer() failed: %v", err)
		}

		// Verify port is in range 3000-3010
		port := server.Port()
		if port < 3000 || port > 3010 {
			t.Errorf("port %d not in range [3000, 3010]", port)
		}

		// Start server
		redirectURI, err := server.Start()
		if err != nil {
			t.Fatalf("Start() failed: %v", err)
		}

		// Verify redirect URI format
		expectedURI := fmt.Sprintf("http://localhost:%d/oauth2callback", port)
		if redirectURI != expectedURI {
			t.Errorf("redirectURI = %s, want %s", redirectURI, expectedURI)
		}

		// Verify server is listening on 127.0.0.1 (not 0.0.0.0)
		// We can't directly test this, but we verify the callback works
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/oauth2callback?code=test123", port))
		if err != nil {
			t.Fatalf("failed to connect to callback server: %v", err)
		}
		resp.Body.Close()

		// Wait for callback result
		code, err := server.WaitForCallback(1 * time.Second)
		if err != nil {
			t.Fatalf("WaitForCallback() failed: %v", err)
		}

		if code != "test123" {
			t.Errorf("code = %s, want test123", code)
		}

		// Stop server
		if err := server.Stop(); err != nil {
			t.Fatalf("Stop() failed: %v", err)
		}

		// Verify port is released (connection should fail)
		time.Sleep(100 * time.Millisecond)
		_, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/oauth2callback", port))
		if err == nil {
			t.Error("expected connection to fail after server stop, but it succeeded")
		}
	})

	t.Run("E2E-021: Concurrent mcp-proxy instances with different URLs", func(t *testing.T) {
		// Create and start first server
		server1, err := oauth.NewCallbackServer()
		if err != nil {
			t.Fatalf("NewCallbackServer() 1 failed: %v", err)
		}

		_, err = server1.Start()
		if err != nil {
			t.Fatalf("Start() 1 failed: %v", err)
		}
		defer server1.Stop()

		port1 := server1.Port()

		// Create and start second server (should get different port)
		server2, err := oauth.NewCallbackServer()
		if err != nil {
			t.Fatalf("NewCallbackServer() 2 failed: %v", err)
		}

		_, err = server2.Start()
		if err != nil {
			t.Fatalf("Start() 2 failed: %v", err)
		}
		defer server2.Stop()

		port2 := server2.Port()

		// Verify different ports
		if port1 == port2 {
			t.Errorf("servers should use different ports, both using %d", port1)
		}

		// Send callbacks to both servers
		go func() {
			http.Get(fmt.Sprintf("http://127.0.0.1:%d/oauth2callback?code=code1", port1))
		}()

		go func() {
			http.Get(fmt.Sprintf("http://127.0.0.1:%d/oauth2callback?code=code2", port2))
		}()

		// Wait for both callbacks
		code1, err := server1.WaitForCallback(1 * time.Second)
		if err != nil {
			t.Fatalf("WaitForCallback() 1 failed: %v", err)
		}

		code2, err := server2.WaitForCallback(1 * time.Second)
		if err != nil {
			t.Fatalf("WaitForCallback() 2 failed: %v", err)
		}

		// Verify correct codes
		if code1 != "code1" {
			t.Errorf("code1 = %s, want code1", code1)
		}

		if code2 != "code2" {
			t.Errorf("code2 = %s, want code2", code2)
		}
	})

	t.Run("E2E-008: User cancels OAuth flow (timeout)", func(t *testing.T) {
		server, err := oauth.NewCallbackServer()
		if err != nil {
			t.Fatalf("NewCallbackServer() failed: %v", err)
		}

		_, err = server.Start()
		if err != nil {
			t.Fatalf("Start() failed: %v", err)
		}
		defer server.Stop()

		// Wait for callback with short timeout (simulate user not completing flow)
		_, err = server.WaitForCallback(100 * time.Millisecond)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}

		// Verify error message contains the expected text
		expectedMsg := "OAuth flow timed out. User did not complete authentication."
		if !contains(err.Error(), expectedMsg) {
			t.Errorf("error message = %q, want to contain %q", err.Error(), expectedMsg)
		}
	})

	t.Run("handles OAuth error in callback", func(t *testing.T) {
		server, err := oauth.NewCallbackServer()
		if err != nil {
			t.Fatalf("NewCallbackServer() failed: %v", err)
		}

		_, err = server.Start()
		if err != nil {
			t.Fatalf("Start() failed: %v", err)
		}
		defer server.Stop()

		// Send error callback
		go func() {
			http.Get(fmt.Sprintf("http://127.0.0.1:%d/oauth2callback?error=access_denied&error_description=User+denied+access", server.Port()))
		}()

		// Wait for callback
		_, err = server.WaitForCallback(1 * time.Second)
		if err == nil {
			t.Fatal("expected error for OAuth error callback, got nil")
		}

		// Verify error contains the OAuth error
		if err.Error() == "" {
			t.Error("error message is empty")
		}
	})

	t.Run("handles missing code in callback", func(t *testing.T) {
		server, err := oauth.NewCallbackServer()
		if err != nil {
			t.Fatalf("NewCallbackServer() failed: %v", err)
		}

		_, err = server.Start()
		if err != nil {
			t.Fatalf("Start() failed: %v", err)
		}
		defer server.Stop()

		// Send callback without code
		go func() {
			http.Get(fmt.Sprintf("http://127.0.0.1:%d/oauth2callback", server.Port()))
		}()

		// Wait for callback
		_, err = server.WaitForCallback(1 * time.Second)
		if err == nil {
			t.Fatal("expected error for missing code, got nil")
		}
	})
}
