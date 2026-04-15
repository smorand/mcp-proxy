package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// PKCEData holds the PKCE code verifier and challenge
type PKCEData struct {
	CodeVerifier  string
	CodeChallenge string
}

// GeneratePKCE generates a cryptographically secure PKCE code verifier and challenge
// The code_verifier is a random string of 43-128 characters, base64url-encoded
// The code_challenge is SHA256(code_verifier), base64url-encoded
func GeneratePKCE() (*PKCEData, error) {
	// Generate 32 random bytes (will be 43 chars when base64url-encoded)
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Base64url-encode the verifier (no padding)
	codeVerifier := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(verifierBytes)

	// Compute SHA256 hash of the verifier
	hash := sha256.Sum256([]byte(codeVerifier))

	// Base64url-encode the challenge (no padding)
	codeChallenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])

	return &PKCEData{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
	}, nil
}
