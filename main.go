package main

import (
	"fmt"

	"github.com/smorand/mcp-proxy/internal/config"
	"github.com/smorand/mcp-proxy/internal/errors"
)

func main() {
	// Parse configuration
	cfg, err := config.Parse()
	if err != nil {
		errors.Fatal(err)
	}

	// For US-001, we just validate the configuration
	// OAuth flow and MCP proxy will be implemented in subsequent stories
	fmt.Printf("Configuration validated successfully:\n")
	fmt.Printf("  Server URL: %s\n", cfg.ServerURL)
	fmt.Printf("  Client ID: %s\n", cfg.ClientID)
	fmt.Printf("  Client Secret: [REDACTED]\n")
}
