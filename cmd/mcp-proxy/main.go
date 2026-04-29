// Command mcp-proxy is an authenticating proxy between a local MCP client
// (over stdio) and a remote MCP server (over HTTPS with OAuth2.1).
package main

import (
	"github.com/smorand/mcp-proxy/internal/app"
	"github.com/smorand/mcp-proxy/internal/apperr"
)

func main() {
	if err := app.Run(); err != nil {
		apperr.Fatal(err)
	}
}
