package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/smorand/mcp-proxy/internal/errors"
)

// Config holds the application configuration
type Config struct {
	ServerURL    string
	ClientID     string
	ClientSecret string
}

// Parse parses command-line arguments and environment variables
func Parse() (*Config, error) {
	var (
		serverURL    string
		clientID     string
		clientSecret string
	)

	// Define flags
	flag.StringVar(&serverURL, "url", "", "MCP server URL (required, must be HTTPS)")
	flag.StringVar(&serverURL, "u", "", "MCP server URL (required, must be HTTPS)")
	flag.StringVar(&clientID, "client-id", "env:GOOGLE_CLIENT_ID", "OAuth2.1 client ID")
	flag.StringVar(&clientID, "i", "env:GOOGLE_CLIENT_ID", "OAuth2.1 client ID")
	flag.StringVar(&clientSecret, "client-secret", "env:GOOGLE_CLIENT_SECRET", "OAuth2.1 client secret")
	flag.StringVar(&clientSecret, "s", "env:GOOGLE_CLIENT_SECRET", "OAuth2.1 client secret")

	flag.Parse()

	// Validate URL is provided
	if serverURL == "" {
		flag.Usage()
		return nil, errors.NewConfigError("MCP server URL is required. Use --url or -u flag", nil)
	}

	// Resolve client_id from environment if needed
	resolvedClientID, err := resolveValue(clientID)
	if err != nil {
		return nil, errors.NewConfigError("failed to resolve client_id", err)
	}
	if resolvedClientID == "" {
		return nil, errors.NewConfigError("client_id is required. Set via --client-id flag or GOOGLE_CLIENT_ID environment variable", nil)
	}

	// Resolve client_secret from environment if needed
	resolvedClientSecret, err := resolveValue(clientSecret)
	if err != nil {
		return nil, errors.NewConfigError("failed to resolve client_secret", err)
	}
	if resolvedClientSecret == "" {
		return nil, errors.NewConfigError("client_secret is required. Set via --client-secret flag or GOOGLE_CLIENT_SECRET environment variable", nil)
	}

	// Validate URL format
	if err := validateURL(serverURL); err != nil {
		return nil, err
	}

	return &Config{
		ServerURL:    serverURL,
		ClientID:     resolvedClientID,
		ClientSecret: resolvedClientSecret,
	}, nil
}

// resolveValue resolves a value that may be an environment variable reference
// If the value starts with "env:", it reads from the environment variable
func resolveValue(value string) (string, error) {
	if strings.HasPrefix(value, "env:") {
		envVar := strings.TrimPrefix(value, "env:")
		return os.Getenv(envVar), nil
	}
	return value, nil
}

// validateURL validates that the URL is well-formed and uses HTTPS
func validateURL(rawURL string) error {
	// Parse URL to validate format first
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return errors.NewConfigError("Invalid MCP server URL format", err)
	}

	// Ensure scheme is present
	if parsedURL.Scheme == "" {
		return errors.NewConfigError("Invalid MCP server URL format", fmt.Errorf("missing scheme"))
	}

	// Check if URL uses http:// (insecure)
	if parsedURL.Scheme == "http" {
		return errors.NewConfigError("MCP server URL must use HTTPS. Insecure HTTP connections are not allowed", nil)
	}

	// Ensure scheme is https
	if parsedURL.Scheme != "https" {
		return errors.NewConfigError("Invalid MCP server URL format", fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme))
	}

	// Ensure host is present
	if parsedURL.Host == "" {
		return errors.NewConfigError("Invalid MCP server URL format", fmt.Errorf("missing host"))
	}

	return nil
}
