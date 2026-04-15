package tests

import (
	"testing"
	"time"

	"github.com/smorand/mcp-proxy/internal/token"
)

func TestTokenExpiration(t *testing.T) {
	t.Run("IsExpired returns false for future expiration", func(t *testing.T) {
		td := &token.TokenData{
			AccessToken:    "test-token",
			ExpirationTime: time.Date(2026, 4, 15, 23, 0, 0, 0, time.UTC),
		}
		now := time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC)

		if td.IsExpired(now) {
			t.Error("expected token to not be expired, but IsExpired returned true")
		}
	})

	t.Run("IsExpired returns true for past expiration", func(t *testing.T) {
		td := &token.TokenData{
			AccessToken:    "expired-token",
			ExpirationTime: time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC),
		}
		now := time.Date(2026, 4, 15, 22, 0, 0, 0, time.UTC)

		if !td.IsExpired(now) {
			t.Error("expected token to be expired, but IsExpired returned false")
		}
	})

	t.Run("IsExpired returns true when time equals expiration", func(t *testing.T) {
		expTime := time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC)
		td := &token.TokenData{
			AccessToken:    "test-token",
			ExpirationTime: expTime,
		}

		if !td.IsExpired(expTime) {
			t.Error("expected token to be expired at exact expiration time, but IsExpired returned false")
		}
	})

	t.Run("HasRefreshToken returns true when refresh token present", func(t *testing.T) {
		td := &token.TokenData{
			AccessToken:  "test-token",
			RefreshToken: "refresh-token",
		}

		if !td.HasRefreshToken() {
			t.Error("expected HasRefreshToken to return true")
		}
	})

	t.Run("HasRefreshToken returns false when refresh token empty", func(t *testing.T) {
		td := &token.TokenData{
			AccessToken:  "test-token",
			RefreshToken: "",
		}

		if td.HasRefreshToken() {
			t.Error("expected HasRefreshToken to return false")
		}
	})
}
