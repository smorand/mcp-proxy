package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smorand/mcp-proxy/internal/oauth"
)

func TestExchangeCodeForToken(t *testing.T) {
	t.Run("E2E-001: First-time OAuth flow with token caching (token exchange part)", func(t *testing.T) {
		// Create mock token server
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request method
			if r.Method != "POST" {
				t.Errorf("method = %s, want POST", r.Method)
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			// Verify Content-Type
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("Content-Type = %s, want application/x-www-form-urlencoded",
					r.Header.Get("Content-Type"))
			}

			// Parse form data
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm() failed: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Verify required parameters
			if r.FormValue("grant_type") != "authorization_code" {
				t.Errorf("grant_type = %s, want authorization_code", r.FormValue("grant_type"))
			}
			if r.FormValue("code") == "" {
				t.Error("code is empty")
			}
			if r.FormValue("redirect_uri") == "" {
				t.Error("redirect_uri is empty")
			}
			if r.FormValue("client_id") == "" {
				t.Error("client_id is empty")
			}
			if r.FormValue("client_secret") == "" {
				t.Error("client_secret is empty")
			}
			if r.FormValue("code_verifier") == "" {
				t.Error("code_verifier is empty")
			}

			// Return token response
			tokenResp := oauth.TokenResponse{
				AccessToken:  "ya29.a0AfH6SMBx_test_access_token",
				RefreshToken: "1//0gHZ9K_test_refresh_token",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tokenResp)
		}))
		defer server.Close()

		// Exchange code for token
		tokenResp, err := oauth.ExchangeCodeForToken(
			server.URL,
			"test-client-id",
			"test-client-secret",
			"test-auth-code",
			"http://localhost:3000/oauth2callback",
			"test-code-verifier",
		)

		if err != nil {
			t.Fatalf("ExchangeCodeForToken() failed: %v", err)
		}

		// Verify token response
		if tokenResp.AccessToken != "ya29.a0AfH6SMBx_test_access_token" {
			t.Errorf("access_token = %s, want ya29.a0AfH6SMBx_test_access_token",
				tokenResp.AccessToken)
		}

		if tokenResp.RefreshToken != "1//0gHZ9K_test_refresh_token" {
			t.Errorf("refresh_token = %s, want 1//0gHZ9K_test_refresh_token",
				tokenResp.RefreshToken)
		}

		if tokenResp.ExpiresIn != 3600 {
			t.Errorf("expires_in = %d, want 3600", tokenResp.ExpiresIn)
		}

		if tokenResp.TokenType != "Bearer" {
			t.Errorf("token_type = %s, want Bearer", tokenResp.TokenType)
		}
	})

	t.Run("E2E-013: OAuth token exchange fails", func(t *testing.T) {
		// Create mock token server that returns error
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "invalid_client",
				"error_description": "Client authentication failed",
			})
		}))
		defer server.Close()

		// Exchange code for token
		_, err := oauth.ExchangeCodeForToken(
			server.URL,
			"invalid-client-id",
			"invalid-client-secret",
			"test-auth-code",
			"http://localhost:3000/oauth2callback",
			"test-code-verifier",
		)

		if err == nil {
			t.Fatal("expected error for invalid client, got nil")
		}

		// Verify error message contains "invalid_client"
		if !strings.Contains(err.Error(), "invalid_client") {
			t.Errorf("error message = %q, want to contain 'invalid_client'", err.Error())
		}
	})

	t.Run("handles missing access_token in response", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"expires_in": 3600,
				"token_type": "Bearer",
			})
		}))
		defer server.Close()

		_, err := oauth.ExchangeCodeForToken(
			server.URL,
			"test-client-id",
			"test-client-secret",
			"test-auth-code",
			"http://localhost:3000/oauth2callback",
			"test-code-verifier",
		)

		if err == nil {
			t.Fatal("expected error for missing access_token, got nil")
		}
	})

	t.Run("handles missing expires_in in response", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-token",
				"token_type":   "Bearer",
			})
		}))
		defer server.Close()

		_, err := oauth.ExchangeCodeForToken(
			server.URL,
			"test-client-id",
			"test-client-secret",
			"test-auth-code",
			"http://localhost:3000/oauth2callback",
			"test-code-verifier",
		)

		if err == nil {
			t.Fatal("expected error for missing expires_in, got nil")
		}
	})

	t.Run("handles optional refresh_token", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-token",
				"expires_in":   3600,
				"token_type":   "Bearer",
				// No refresh_token
			})
		}))
		defer server.Close()

		tokenResp, err := oauth.ExchangeCodeForToken(
			server.URL,
			"test-client-id",
			"test-client-secret",
			"test-auth-code",
			"http://localhost:3000/oauth2callback",
			"test-code-verifier",
		)

		if err != nil {
			t.Fatalf("ExchangeCodeForToken() failed: %v", err)
		}

		// Verify refresh_token is empty (optional field)
		if tokenResp.RefreshToken != "" {
			t.Errorf("refresh_token = %s, want empty string", tokenResp.RefreshToken)
		}
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		_, err := oauth.ExchangeCodeForToken(
			server.URL,
			"test-client-id",
			"test-client-secret",
			"test-auth-code",
			"http://localhost:3000/oauth2callback",
			"test-code-verifier",
		)

		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})
}
