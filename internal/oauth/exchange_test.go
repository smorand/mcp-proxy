package oauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smorand/mcp-proxy/internal/oauth"
)

func TestExchangeCodeForToken(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		wantErr   bool
		errSubstr string
		access    string
		refresh   string
	}{
		{
			name: "successful exchange",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Errorf("ParseForm: %v", err)
				}
				if r.FormValue("grant_type") != "authorization_code" {
					t.Errorf("grant_type = %q", r.FormValue("grant_type"))
				}
				if r.FormValue("code") != "auth-code" {
					t.Errorf("code = %q", r.FormValue("code"))
				}
				if r.FormValue("code_verifier") != "verifier" {
					t.Errorf("code_verifier = %q", r.FormValue("code_verifier"))
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"access_token": "AT-1",
					"refresh_token": "RT-1",
					"expires_in": 3600,
					"token_type": "Bearer"
				}`))
			},
			access:  "AT-1",
			refresh: "RT-1",
		},
		{
			name: "server returns 400 with error JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"bad code"}`))
			},
			wantErr:   true,
			errSubstr: "invalid_grant",
		},
		{
			name: "missing access_token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"expires_in": 3600}`))
			},
			wantErr:   true,
			errSubstr: "access_token",
		},
		{
			name: "missing expires_in",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"access_token":"AT-1"}`))
			},
			wantErr:   true,
			errSubstr: "expires_in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(tt.handler)
			defer server.Close()

			resp, err := oauth.ExchangeCodeForToken(
				context.Background(),
				server.Client(),
				server.URL,
				"client-id",
				"client-secret",
				"auth-code",
				"http://localhost:3000/oauth2callback",
				"verifier",
			)
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
			if resp.AccessToken != tt.access {
				t.Errorf("AccessToken = %q, want %q", resp.AccessToken, tt.access)
			}
			if resp.RefreshToken != tt.refresh {
				t.Errorf("RefreshToken = %q, want %q", resp.RefreshToken, tt.refresh)
			}
		})
	}
}
