package oauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smorand/mcp-proxy/internal/oauth"
)

func TestDiscoverEndpoints(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		wantErr   bool
		errSubstr string
		authEnd   string
		tokenEnd  string
	}{
		{
			name: "successful discovery",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/.well-known/oauth-authorization-server" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"authorization_endpoint": "https://auth.example.com/authorize",
					"token_endpoint": "https://auth.example.com/token",
					"issuer": "https://auth.example.com"
				}`))
			},
			authEnd:  "https://auth.example.com/authorize",
			tokenEnd: "https://auth.example.com/token",
		},
		{
			name: "404 from server",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			},
			wantErr:   true,
			errSubstr: "status 404",
		},
		{
			name: "missing authorization_endpoint",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"token_endpoint": "https://auth.example.com/token"}`))
			},
			wantErr:   true,
			errSubstr: "authorization_endpoint",
		},
		{
			name: "missing token_endpoint",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"authorization_endpoint": "https://auth.example.com/authorize"}`))
			},
			wantErr:   true,
			errSubstr: "token_endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(tt.handler)
			defer server.Close()

			discovery, err := oauth.DiscoverEndpoints(context.Background(), server.URL, server.Client())
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if discovery.AuthorizationEndpoint != tt.authEnd {
				t.Errorf("AuthorizationEndpoint = %q, want %q", discovery.AuthorizationEndpoint, tt.authEnd)
			}
			if discovery.TokenEndpoint != tt.tokenEnd {
				t.Errorf("TokenEndpoint = %q, want %q", discovery.TokenEndpoint, tt.tokenEnd)
			}
		})
	}
}
