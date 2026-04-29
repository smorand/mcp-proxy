package oauth_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/smorand/mcp-proxy/internal/oauth"
)

func TestCallbackServer_HappyPath(t *testing.T) {
	server, err := oauth.NewCallbackServer()
	if err != nil {
		t.Fatalf("NewCallbackServer: %v", err)
	}
	redirectURI, err := server.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer server.Stop()

	if !strings.HasPrefix(redirectURI, "http://localhost:") {
		t.Errorf("redirectURI = %q", redirectURI)
	}
	if !strings.HasSuffix(redirectURI, "/oauth2callback") {
		t.Errorf("redirectURI = %q", redirectURI)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		resp, err := http.Get(redirectURI + "?code=auth-code")
		if err != nil {
			return
		}
		resp.Body.Close()
	}()

	code, err := server.WaitForCallback(2 * time.Second)
	if err != nil {
		t.Fatalf("WaitForCallback: %v", err)
	}
	if code != "auth-code" {
		t.Errorf("code = %q, want auth-code", code)
	}
}

func TestCallbackServer_OAuthError(t *testing.T) {
	server, err := oauth.NewCallbackServer()
	if err != nil {
		t.Fatalf("NewCallbackServer: %v", err)
	}
	redirectURI, err := server.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer server.Stop()

	go func() {
		time.Sleep(50 * time.Millisecond)
		resp, err := http.Get(redirectURI + "?error=access_denied&error_description=user+denied")
		if err != nil {
			return
		}
		resp.Body.Close()
	}()

	_, err = server.WaitForCallback(2 * time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("error %q missing access_denied", err.Error())
	}
}

func TestCallbackServer_Timeout(t *testing.T) {
	server, err := oauth.NewCallbackServer()
	if err != nil {
		t.Fatalf("NewCallbackServer: %v", err)
	}
	if _, err := server.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer server.Stop()

	_, err = server.WaitForCallback(100 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error %q missing timed out", err.Error())
	}
}
