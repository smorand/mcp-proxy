# mcp-proxy

A command-line tool that acts as an authentication proxy for MCP (Model Context Protocol) servers. It enables developers to use MCP servers that require OAuth2.1 authentication by handling the authentication flow transparently.

## Features

- **OAuth2.1 with PKCE**: Secure authentication flow with Proof Key for Code Exchange
- **Automatic OAuth discovery**: Discovers OAuth endpoints via .well-known mechanism
- **Browser-based authentication**: Opens system browser for user authentication
- **Automatic token refresh**: Seamlessly refreshes expired tokens using refresh_token
- **Automatic token management**: Caches tokens and handles expiration
- **Secure token storage**: Tokens stored locally with proper file permissions (0600)
- **HTTPS enforcement**: Only allows secure HTTPS connections to MCP servers
- **Environment variable support**: Configure credentials via environment variables
- **Cross-platform**: Supports macOS, Linux, and Windows

## Prerequisites

- Go 1.21+ (for building from source)
- OAuth2.1 credentials (client_id and client_secret) from your OAuth provider

## Installation

### From Source

```bash
git clone https://github.com/smorand/mcp-proxy.git
cd mcp-proxy
make build
```

The binary will be created as `mcp-proxy` in the current directory. Move it to a directory in your PATH:

```bash
sudo mv mcp-proxy /usr/local/bin/
```

## Usage

### Basic Usage

```bash
mcp-proxy -u https://mcp.example.com
```

This will:
1. Read OAuth credentials from environment variables (GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET)
2. Check for cached tokens in `~/.cache/mcp-proxy/`
3. If a cached token exists but is expired, automatically refresh it using the stored refresh_token
4. If refresh fails or no valid token exists:
   - Discover OAuth endpoints via .well-known mechanism
   - Generate PKCE codes for secure authentication
   - Open your browser for authentication
   - Exchange authorization code for access token
   - Cache the token for future use
5. (In future versions) Proxy MCP requests using the access token

### Custom Credentials

You can provide credentials directly via command-line flags:

```bash
mcp-proxy -u https://mcp.example.com -i my-client-id -s my-secret
```

### Environment Variables

Set credentials as environment variables:

```bash
export GOOGLE_CLIENT_ID="your-client-id"
export GOOGLE_CLIENT_SECRET="your-client-secret"
mcp-proxy -u https://mcp.example.com
```

You can also reference custom environment variables:

```bash
export MY_CLIENT_ID="your-client-id"
export MY_SECRET="your-client-secret"
mcp-proxy -u https://mcp.example.com -i env:MY_CLIENT_ID -s env:MY_SECRET
```

## Configuration

### Command-Line Flags

- `--url`, `-u`: MCP server URL (required, must be HTTPS)
- `--client-id`, `-i`: OAuth2.1 client ID (default: `env:GOOGLE_CLIENT_ID`)
- `--client-secret`, `-s`: OAuth2.1 client secret (default: `env:GOOGLE_CLIENT_SECRET`)

### Token Storage

Tokens are stored in `~/.cache/mcp-proxy/` with the following security measures:
- Directory permissions: 0700 (owner read/write/execute only)
- File permissions: 0600 (owner read/write only)
- Filenames are base64url-encoded server URLs

## Development

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Code Quality

```bash
make check  # Runs format, lint, and test
```

### Available Make Targets

- `make build`: Build the binary
- `make test`: Run all tests
- `make format`: Format Go code
- `make lint`: Run linter (requires golangci-lint)
- `make check`: Run all quality checks
- `make clean`: Clean build artifacts

## Troubleshooting

### Error: client_id is required

Make sure you've set the GOOGLE_CLIENT_ID environment variable or provided it via the `--client-id` flag.

```bash
export GOOGLE_CLIENT_ID="your-client-id"
```

### Error: client_secret is required

Make sure you've set the GOOGLE_CLIENT_SECRET environment variable or provided it via the `--client-secret` flag.

```bash
export GOOGLE_CLIENT_SECRET="your-client-secret"
```

### Error: MCP server URL must use HTTPS

The tool only accepts HTTPS URLs for security reasons. Make sure your URL starts with `https://`.

### Error: Invalid MCP server URL format

Check that your URL is well-formed. It should be a complete URL like `https://mcp.example.com`.

## Project Status

**Current Version**: US-003 (Token Refresh & Expiration)

Completed features:
- ✅ CLI argument parsing with environment variable support (US-001)
- ✅ Secure token file management (US-001)
- ✅ Error handling framework (US-001)
- ✅ OAuth2.1 discovery via .well-known endpoints (US-002)
- ✅ PKCE code generation with SHA256 (US-002)
- ✅ Browser-based authentication flow (US-002)
- ✅ Token exchange and caching (US-002)
- ✅ Token expiration detection (US-003)
- ✅ Automatic token refresh via refresh_token (US-003)
- ✅ Fallback to full OAuth when refresh fails (US-003)
- ⏳ MCP server proxy with stdio interface (coming in US-004)

## License

MIT License

## Contributing

Contributions are welcome! Please ensure all tests pass before submitting a pull request:

```bash
make check
```
