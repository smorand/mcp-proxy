package tests

import (
	"testing"

	"github.com/smorand/mcp-proxy/internal/oauth"
)

func TestGeneratePKCE(t *testing.T) {
	t.Run("generates valid PKCE codes", func(t *testing.T) {
		pkce, err := oauth.GeneratePKCE()
		if err != nil {
			t.Fatalf("GeneratePKCE() failed: %v", err)
		}

		// Verify code_verifier length (43-128 chars for base64url)
		if len(pkce.CodeVerifier) < 43 || len(pkce.CodeVerifier) > 128 {
			t.Errorf("code_verifier length %d not in range [43, 128]", len(pkce.CodeVerifier))
		}

		// Verify code_challenge is not empty
		if pkce.CodeChallenge == "" {
			t.Error("code_challenge is empty")
		}

		// Verify code_challenge length (SHA256 base64url is 43 chars)
		if len(pkce.CodeChallenge) != 43 {
			t.Errorf("code_challenge length %d, expected 43", len(pkce.CodeChallenge))
		}
	})

	t.Run("generates unique codes on each call", func(t *testing.T) {
		pkce1, err := oauth.GeneratePKCE()
		if err != nil {
			t.Fatalf("GeneratePKCE() failed: %v", err)
		}

		pkce2, err := oauth.GeneratePKCE()
		if err != nil {
			t.Fatalf("GeneratePKCE() failed: %v", err)
		}

		if pkce1.CodeVerifier == pkce2.CodeVerifier {
			t.Error("code_verifier should be unique on each call")
		}

		if pkce1.CodeChallenge == pkce2.CodeChallenge {
			t.Error("code_challenge should be unique on each call")
		}
	})
}
