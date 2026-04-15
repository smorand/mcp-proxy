package token

import "time"

// IsExpired checks whether the token has expired based on the given current time.
// Returns true if the token's expiration time is at or before the current time.
func (t *TokenData) IsExpired(now time.Time) bool {
	return !now.Before(t.ExpirationTime)
}

// HasRefreshToken checks whether the token data contains a refresh token.
func (t *TokenData) HasRefreshToken() bool {
	return t.RefreshToken != ""
}
