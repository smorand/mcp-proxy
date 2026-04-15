# mcp-proxy — AI Development Guide

## Project Overview

**mcp-proxy** is a Go-based CLI tool that acts as an authentication proxy for MCP (Model Context Protocol) servers. It handles OAuth2.1 authentication with PKCE, automatic token refresh, and secure token storage.

**Tech Stack:**
- Language: Go 1.21+
- CLI: Standard library `flag` package
- Testing: Go testing framework with table-driven tests
- Build: Makefile

## Key Commands

```bash
# Build
make build

# Test
make test

# Format code
make format

# Run all quality checks
make check

# Clean artifacts
make clean
```

## Project Structure

```
mcp-proxy/
├── main.go                    # Entry point with OAuth flow
├── internal/
│   ├── config/               # CLI parsing & validation
│   ├── token/                # Token file I/O, expiration & refresh
│   │   ├── storage.go        # Token persistence (atomic writes)
│   │   ├── expiration.go     # Expiration detection
│   │   └── refresh.go        # Token refresh via refresh_token
│   ├── oauth/                # OAuth2.1 flow components
│   │   ├── pkce.go          # PKCE code generation
│   │   ├── discovery.go     # .well-known endpoint discovery
│   │   ├── callback.go      # HTTP callback server
│   │   ├── browser.go       # Browser integration
│   │   └── exchange.go      # Token exchange
│   └── errors/               # Error types & exit codes
├── tests/                    # E2E tests
├── user-stories/             # Implementation backlog
└── specs/                    # Specifications
```

## Essential Conventions

### Error Handling
- All errors start with "Error: "
- Use specific exit codes (1-5):
  - 1: Configuration error
  - 2: Authentication error
  - 3: Network error
  - 4: File system error
  - 5: Token error
- Never include sensitive data in error messages

### Configuration
- Support both CLI flags and environment variables
- Use "env:" prefix for environment variable references
- Default credentials: `env:GOOGLE_CLIENT_ID`, `env:GOOGLE_CLIENT_SECRET`

### Token Storage
- Location: `~/.cache/mcp-proxy/{base64url(server_url)}.json`
- Directory permissions: 0700
- File permissions: 0600
- Atomic writes: temp file + rename
- Always read from disk (no in-memory caching)

### Testing
- Table-driven tests
- E2E tests run the actual binary
- Test IDs match spec (E2E-XXX)
- Comprehensive edge case coverage

## Documentation Index

- **README.md**: User-facing documentation
- **user-stories/**: Implementation backlog (US-001 through US-004)
- **specs/**: Technical specifications

## Current Status

**Implemented:** US-001 (Foundation) + US-002 (OAuth2.1 Flow) + US-003 (Token Refresh)
- ✅ CLI argument parsing
- ✅ Token file management
- ✅ Error handling framework
- ✅ OAuth2.1 discovery via .well-known
- ✅ PKCE code generation (SHA256)
- ✅ HTTP callback server (ports 3000-3010)
- ✅ Browser integration (macOS/Linux/Windows)
- ✅ Token exchange and caching
- ✅ Token expiration detection
- ✅ Automatic token refresh via refresh_token
- ✅ Fallback to full OAuth when refresh fails (400/401)

**Next:** US-004 (MCP Proxy Streaming)
