// Package config parses CLI flags and environment variables.
package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/smorand/mcp-proxy/internal/apperr"
)

const envPrefix = "env:"

// Config holds runtime configuration for the proxy.
type Config struct {
	ServerURL    string
	ClientID     string
	ClientSecret string
}

// Parse parses os.Args into a Config. CLI flags override environment
// variables. Values prefixed with "env:" are resolved from the environment.
func Parse() (*Config, error) {
	var (
		serverURL    string
		clientID     string
		clientSecret string
	)

	flag.StringVar(&serverURL, "url", "", "MCP server URL (required, must be HTTPS)")
	flag.StringVar(&serverURL, "u", "", "MCP server URL (required, must be HTTPS)")
	flag.StringVar(&clientID, "client-id", "env:GOOGLE_CLIENT_ID", "OAuth2.1 client ID")
	flag.StringVar(&clientID, "i", "env:GOOGLE_CLIENT_ID", "OAuth2.1 client ID")
	flag.StringVar(&clientSecret, "client-secret", "env:GOOGLE_CLIENT_SECRET", "OAuth2.1 client secret")
	flag.StringVar(&clientSecret, "s", "env:GOOGLE_CLIENT_SECRET", "OAuth2.1 client secret")

	flag.Parse()

	if serverURL == "" {
		flag.Usage()
		return nil, apperr.NewConfigError("MCP server URL is required. Use --url or -u flag", nil)
	}

	resolvedClientID := resolveValue(clientID)
	if resolvedClientID == "" {
		return nil, apperr.NewConfigError("client_id is required. Set via --client-id flag or GOOGLE_CLIENT_ID environment variable", nil)
	}

	resolvedClientSecret := resolveValue(clientSecret)
	if resolvedClientSecret == "" {
		return nil, apperr.NewConfigError("client_secret is required. Set via --client-secret flag or GOOGLE_CLIENT_SECRET environment variable", nil)
	}

	if err := validateURL(serverURL); err != nil {
		return nil, err
	}

	return &Config{
		ServerURL:    serverURL,
		ClientID:     resolvedClientID,
		ClientSecret: resolvedClientSecret,
	}, nil
}

func resolveValue(value string) string {
	if name, ok := strings.CutPrefix(value, envPrefix); ok {
		return os.Getenv(name)
	}
	return value
}

func validateURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return apperr.NewConfigError("Invalid MCP server URL format", err)
	}
	if parsedURL.Scheme == "" {
		return apperr.NewConfigError("Invalid MCP server URL format", fmt.Errorf("missing scheme"))
	}
	if parsedURL.Scheme == "http" {
		return apperr.NewConfigError("MCP server URL must use HTTPS. Insecure HTTP connections are not allowed", nil)
	}
	if parsedURL.Scheme != "https" {
		return apperr.NewConfigError("Invalid MCP server URL format", fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme))
	}
	if parsedURL.Host == "" {
		return apperr.NewConfigError("Invalid MCP server URL format", fmt.Errorf("missing host"))
	}
	return nil
}
