package tests

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smorand/mcp-proxy/internal/token"
)

// TestE2E003_AutomaticTokenRefresh tests automatic token refresh with valid refresh_token.
func TestE2E003_AutomaticTokenRefresh(t *testing.T) {
	// Create mock OAuth server that accepts refresh requests
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify grant_type is refresh_token
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %s, want refresh_token", r.FormValue("grant_type"))
		}

		// Verify refresh_token is present
		if r.FormValue("refresh_token") != "valid-refresh" {
			t.Errorf("refresh_token = %s, want valid-refresh", r.FormValue("refresh_token"))
		}

		// Verify client credentials
		if r.FormValue("client_id") == "" {
			t.Error("client_id is empty")
		}
		if r.FormValue("client_secret") == "" {
			t.Error("client_secret is empty")
		}

		// Return new token response
		resp := token.RefreshResponse{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	refreshResp, err := token.RefreshAccessToken(
		server.URL,
		"test-client-id",
		"test-client-secret",
		"valid-refresh",
	)

	if err != nil {
		t.Fatalf("RefreshAccessToken() failed: %v", err)
	}

	if refreshResp.AccessToken != "new-access-token" {
		t.Errorf("access_token = %s, want new-access-token", refreshResp.AccessToken)
	}
	if refreshResp.RefreshToken != "new-refresh-token" {
		t.Errorf("refresh_token = %s, want new-refresh-token", refreshResp.RefreshToken)
	}
	if refreshResp.ExpiresIn != 3600 {
		t.Errorf("expires_in = %d, want 3600", refreshResp.ExpiresIn)
	}
}

// TestE2E004_ReAuthWhenRefreshInvalid tests that invalid refresh tokens trigger fallback.
func TestE2E004_ReAuthWhenRefreshInvalid(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "Token has been revoked",
		})
	}))
	defer server.Close()

	_, err := token.RefreshAccessToken(
		server.URL,
		"test-client-id",
		"test-client-secret",
		"invalid-refresh",
	)

	if err == nil {
		t.Fatal("expected error for invalid refresh token, got nil")
	}

	if !errors.Is(err, token.ErrRefreshRejected) {
		t.Errorf("expected ErrRefreshRejected, got: %v", err)
	}
}

// TestE2E011_RefreshTokenRejected tests 401 rejection triggers fallback.
func TestE2E011_RefreshTokenRejected(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid_client",
		})
	}))
	defer server.Close()

	_, err := token.RefreshAccessToken(
		server.URL,
		"test-client-id",
		"test-client-secret",
		"rejected-refresh",
	)

	if err == nil {
		t.Fatal("expected error for rejected refresh token, got nil")
	}

	if !errors.Is(err, token.ErrRefreshRejected) {
		t.Errorf("expected ErrRefreshRejected, got: %v", err)
	}
}

// TestE2E012_NetworkErrorDuringRefresh tests network error handling during refresh.
func TestE2E012_NetworkErrorDuringRefresh(t *testing.T) {
	// Use a server that is immediately closed (unreachable)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	_, err := token.RefreshAccessToken(
		serverURL,
		"test-client-id",
		"test-client-secret",
		"some-refresh-token",
	)

	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}

	// Verify the error message indicates a connection failure
	if !strings.Contains(err.Error(), "Failed to refresh token") {
		t.Errorf("error = %q, want to contain 'Failed to refresh token'", err.Error())
	}
}

// TestE2E018_TokenFileUpdatedAfterRefresh tests that the token file is correctly updated after refresh.
func TestE2E018_TokenFileUpdatedAfterRefresh(t *testing.T) {
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	serverURL := "https://mcp-refresh-test.example.com"

	// Save initial expired token
	err = storage.Save(serverURL, "old-access-token", "old-refresh-token", 1)
	if err != nil {
		t.Fatalf("Failed to save initial token: %v", err)
	}

	// Wait for token to expire
	time.Sleep(2 * time.Second)

	// Verify token is expired
	tokenData, err := storage.Load(serverURL)
	if err != nil {
		t.Fatalf("Failed to load token: %v", err)
	}
	if !tokenData.IsExpired(time.Now()) {
		t.Fatal("expected token to be expired")
	}

	// Mock a successful refresh by saving new tokens
	err = storage.Save(serverURL, "new-access-token", "new-refresh-token", 3600)
	if err != nil {
		t.Fatalf("Failed to save refreshed token: %v", err)
	}

	// Verify the file was updated
	updatedToken, err := storage.Load(serverURL)
	if err != nil {
		t.Fatalf("Failed to load updated token: %v", err)
	}

	if updatedToken.AccessToken != "new-access-token" {
		t.Errorf("access_token = %s, want new-access-token", updatedToken.AccessToken)
	}
	if updatedToken.RefreshToken != "new-refresh-token" {
		t.Errorf("refresh_token = %s, want new-refresh-token", updatedToken.RefreshToken)
	}
	if updatedToken.IsExpired(time.Now()) {
		t.Error("updated token should not be expired")
	}

	// Verify file permissions remain 0600
	cacheDir := storage.GetCacheDir()
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("Failed to read cache dir: %v", err)
	}
	for _, f := range files {
		fp := filepath.Join(cacheDir, f.Name())
		info, statErr := os.Stat(fp)
		if statErr != nil {
			continue
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("file %s permissions = %o, want 0600", f.Name(), info.Mode().Perm())
		}
	}

	// Cleanup
	for _, f := range files {
		os.Remove(filepath.Join(cacheDir, f.Name()))
	}
}

// TestE2E030_AtomicTokenFileWrites tests that token writes are atomic (temp + rename).
func TestE2E030_AtomicTokenFileWrites(t *testing.T) {
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	serverURL := "https://mcp-atomic-test.example.com"

	// Save initial token
	err = storage.Save(serverURL, "initial-token", "initial-refresh", 3600)
	if err != nil {
		t.Fatalf("Failed to save initial token: %v", err)
	}

	cacheDir := storage.GetCacheDir()
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("Failed to read cache dir: %v", err)
	}

	var tokenFile string
	for _, f := range files {
		if strings.Contains(f.Name(), "mcp-atomic-test") || strings.HasSuffix(f.Name(), ".json") {
			tokenFile = filepath.Join(cacheDir, f.Name())
			break
		}
	}

	if tokenFile == "" {
		t.Fatal("Token file not found")
	}
	defer os.Remove(tokenFile)

	// Update token (simulating refresh save)
	err = storage.Save(serverURL, "refreshed-token", "refreshed-refresh", 7200)
	if err != nil {
		t.Fatalf("Failed to save refreshed token: %v", err)
	}

	// Verify no temp files remain
	files, err = os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("Failed to read cache dir after save: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".tmp") {
			t.Errorf("temporary file still exists: %s", f.Name())
		}
	}

	// Verify final content is valid JSON with new values
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("Failed to read token file: %v", err)
	}

	var td token.TokenData
	if err := json.Unmarshal(data, &td); err != nil {
		t.Fatalf("Token file is not valid JSON: %v", err)
	}
	if td.AccessToken != "refreshed-token" {
		t.Errorf("access_token = %s, want refreshed-token", td.AccessToken)
	}
}

// TestRefreshAccessToken_MissingAccessToken tests missing access_token in refresh response.
func TestRefreshAccessToken_MissingAccessToken(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"expires_in": 3600,
			"token_type": "Bearer",
		})
	}))
	defer server.Close()

	_, err := token.RefreshAccessToken(server.URL, "cid", "csecret", "refresh")
	if err == nil {
		t.Fatal("expected error for missing access_token, got nil")
	}
}

// TestRefreshAccessToken_MissingExpiresIn tests missing expires_in in refresh response.
func TestRefreshAccessToken_MissingExpiresIn(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"token_type":   "Bearer",
		})
	}))
	defer server.Close()

	_, err := token.RefreshAccessToken(server.URL, "cid", "csecret", "refresh")
	if err == nil {
		t.Fatal("expected error for missing expires_in, got nil")
	}
}

// TestRefreshAccessToken_OptionalNewRefreshToken tests that refresh_token in response is optional.
func TestRefreshAccessToken_OptionalNewRefreshToken(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
			"token_type":   "Bearer",
		})
	}))
	defer server.Close()

	resp, err := token.RefreshAccessToken(server.URL, "cid", "csecret", "refresh")
	if err != nil {
		t.Fatalf("RefreshAccessToken() failed: %v", err)
	}

	if resp.RefreshToken != "" {
		t.Errorf("expected empty refresh_token, got %q", resp.RefreshToken)
	}
	if resp.AccessToken != "new-token" {
		t.Errorf("access_token = %s, want new-token", resp.AccessToken)
	}
}

// TestRefreshAccessToken_InvalidJSON tests handling of invalid JSON in refresh response.
func TestRefreshAccessToken_InvalidJSON(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	_, err := token.RefreshAccessToken(server.URL, "cid", "csecret", "refresh")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestRefreshAccessToken_ServerError tests handling of 500 server error.
func TestRefreshAccessToken_ServerError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := token.RefreshAccessToken(server.URL, "cid", "csecret", "refresh")
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}

	// 500 should NOT be treated as ErrRefreshRejected (only 400/401 are)
	if errors.Is(err, token.ErrRefreshRejected) {
		t.Error("500 error should not be ErrRefreshRejected")
	}
}

// TestRefreshAccessToken_CorrectContentType verifies the request uses form-urlencoded content type.
func TestRefreshAccessToken_CorrectContentType(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %s, want application/x-www-form-urlencoded", ct)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token.RefreshResponse{
			AccessToken: "token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	}))
	defer server.Close()

	_, err := token.RefreshAccessToken(server.URL, "cid", "csecret", "refresh")
	if err != nil {
		t.Fatalf("RefreshAccessToken() failed: %v", err)
	}
}
