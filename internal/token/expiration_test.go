package token_test

import (
	"testing"
	"time"

	"github.com/smorand/mcp-proxy/internal/token"
)

func TestTokenData_IsExpired(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		expiration time.Time
		want       bool
	}{
		{"future expiration", now.Add(time.Hour), false},
		{"past expiration", now.Add(-time.Hour), true},
		{"exact now", now, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := &token.TokenData{ExpirationTime: tt.expiration}
			if got := td.IsExpired(now); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenData_HasRefreshToken(t *testing.T) {
	tests := []struct {
		name    string
		refresh string
		want    bool
	}{
		{"has refresh", "refresh-token-123", true},
		{"no refresh", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := &token.TokenData{RefreshToken: tt.refresh}
			if got := td.HasRefreshToken(); got != tt.want {
				t.Errorf("HasRefreshToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
