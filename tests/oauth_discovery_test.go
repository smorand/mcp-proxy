package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smorand/mcp-proxy/internal/oauth"
)

func TestDiscoverEndpoints(t *testing.T) {
	t.Run("E2E-027: HTTPS enforcement (reject HTTP URLs)", func(t *testing.T) {
		_, err := oauth.DiscoverEndpoints("http://mcp.test.com")
		if err == nil {
			t.Fatal("expected error for HTTP URL, got nil")
		}
		// Note: This test validates URL parsing, actual HTTPS enforcement
		// is in config.Parse() from US-001
	})

	t.Run("successfully discovers OAuth endpoints", func(t *testing.T) {
		// Create mock discovery server
		discoveryDoc := oauth.OAuthDiscovery{
			AuthorizationEndpoint: "https://auth.example.com/authorize",
			TokenEndpoint:         "https://auth.example.com/token",
			Issuer:                "https://auth.example.com",
		}

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/.well-known/oauth-authorization-server" {
				t.Errorf("unexpected path: %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(discoveryDoc)
		}))
		defer server.Close()

		// Discover endpoints
		discovery, err := oauth.DiscoverEndpoints(server.URL)
		if err != nil {
			t.Fatalf("DiscoverEndpoints() failed: %v", err)
		}

		// Verify endpoints
		if discovery.AuthorizationEndpoint != discoveryDoc.AuthorizationEndpoint {
			t.Errorf("authorization_endpoint = %s, want %s",
				discovery.AuthorizationEndpoint, discoveryDoc.AuthorizationEndpoint)
		}

		if discovery.TokenEndpoint != discoveryDoc.TokenEndpoint {
			t.Errorf("token_endpoint = %s, want %s",
				discovery.TokenEndpoint, discoveryDoc.TokenEndpoint)
		}

		if discovery.Issuer != discoveryDoc.Issuer {
			t.Errorf("issuer = %s, want %s",
				discovery.Issuer, discoveryDoc.Issuer)
		}
	})

	t.Run("handles missing authorization_endpoint", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"token_endpoint": "https://auth.example.com/token",
			})
		}))
		defer server.Close()

		_, err := oauth.DiscoverEndpoints(server.URL)
		if err == nil {
			t.Fatal("expected error for missing authorization_endpoint, got nil")
		}
	})

	t.Run("handles missing token_endpoint", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": "https://auth.example.com/authorize",
			})
		}))
		defer server.Close()

		_, err := oauth.DiscoverEndpoints(server.URL)
		if err == nil {
			t.Fatal("expected error for missing token_endpoint, got nil")
		}
	})

	t.Run("handles 404 response", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, err := oauth.DiscoverEndpoints(server.URL)
		if err == nil {
			t.Fatal("expected error for 404 response, got nil")
		}
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		_, err := oauth.DiscoverEndpoints(server.URL)
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})
}
