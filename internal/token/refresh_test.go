package token_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smorand/mcp-proxy/internal/token"
)

func TestRefreshAccessToken(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    bool
		isRejected bool
		errSubstr  string
		access     string
		refresh    string
	}{
		{
			name: "successful refresh",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Errorf("ParseForm: %v", err)
				}
				if r.FormValue("grant_type") != "refresh_token" {
					t.Errorf("grant_type = %q", r.FormValue("grant_type"))
				}
				_, _ = w.Write([]byte(`{
					"access_token": "AT-2",
					"refresh_token": "RT-2",
					"expires_in": 3600,
					"token_type": "Bearer"
				}`))
			},
			access:  "AT-2",
			refresh: "RT-2",
		},
		{
			name: "rotation: server omits refresh_token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{
					"access_token": "AT-3",
					"expires_in": 3600
				}`))
			},
			access: "AT-3",
		},
		{
			name: "401 rejected",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr:    true,
			isRejected: true,
		},
		{
			name: "400 rejected",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			wantErr:    true,
			isRejected: true,
		},
		{
			name: "500 internal error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:   true,
			errSubstr: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(tt.handler)
			defer server.Close()

			resp, err := token.RefreshAccessToken(
				context.Background(),
				server.Client(),
				server.URL,
				"client-id",
				"client-secret",
				"refresh-token",
			)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.isRejected && !errors.Is(err, token.ErrRefreshRejected) {
					t.Errorf("error = %v, want ErrRefreshRejected", err)
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
