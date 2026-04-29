package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const pkceVerifierEntropy = 32 // bytes -> 43 base64url chars

// PKCEData holds a PKCE code verifier and matching challenge.
type PKCEData struct {
	CodeVerifier  string
	CodeChallenge string
}

// GeneratePKCE returns a fresh PKCE pair. The verifier is 32 random bytes
// base64url-encoded (43 chars); the challenge is SHA256(verifier)
// base64url-encoded (43 chars).
func GeneratePKCE() (*PKCEData, error) {
	verifierBytes := make([]byte, pkceVerifierEntropy)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	codeVerifier := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(verifierBytes)
	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])

	return &PKCEData{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
	}, nil
}
