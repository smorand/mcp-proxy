# US-002: OAuth2.1 Flow with PKCE

> Parent Spec: specs/2026-04-15_22:30:35-mcp-proxy.md
> Status: ready
> Priority: 2
> Depends On: US-001
> Complexity: L

## Objective

Implement the complete OAuth2.1 authorization code flow with PKCE (Proof Key for Code Exchange) support. This includes .well-known endpoint discovery, cryptographically secure PKCE code generation, a temporary HTTP callback server for receiving authorization codes, browser integration, and token exchange. This story enables mcp-proxy to authenticate users and obtain access tokens from OAuth2.1 providers.

## Technical Context

### Stack
- **Language:** Go 1.21+
- **HTTP Server:** Standard library `net/http`
- **Crypto:** Standard library `crypto/rand`, `crypto/sha256`
- **Encoding:** Standard library `encoding/base64`
- **Browser:** `os/exec` with platform-specific commands (macOS: `open`, Linux: `xdg-open`, Windows: `start`)
- **Testing:** Go testing framework with mock HTTP servers (`httptest`)

### Relevant File Structure
```
mcp-proxy/
├── internal/
│   ├── oauth/
│   │   ├── discovery.go      # .well-known endpoint discovery
│   │   ├── pkce.go           # PKCE code generation
│   │   ├── callback.go       # HTTP callback server
│   │   ├── browser.go        # Browser integration
│   │   └── exchange.go       # Token exchange
│   └── config/
│       └── config.go         # (from US-001)
├── tests/
│   ├── oauth_discovery_test.go
│   ├── oauth_pkce_test.go
│   ├── oauth_callback_test.go
│   └── oauth_exchange_test.go
└── go.mod
```

### Existing Patterns
From US-001:
- Error handling with specific exit codes
- Configuration object with resolved credentials
- Token file structure (will be populated by this story)

New patterns to establish:
- **PKCE generation:** Use `crypto/rand` for code_verifier (43-128 chars, base64url), SHA256 for code_challenge
- **HTTP server lifecycle:** Start on demand, stop after callback, try ports 3000-3010
- **Browser integration:** Platform-specific commands, handle errors gracefully
- **Network timeouts:** 5 minutes for OAuth flow, 10 seconds for HTTP requests

### Data Model (excerpt)

**OAuth Discovery Document:**
```go
type OAuthDiscovery struct {
    AuthorizationEndpoint string `json:"authorization_endpoint"`
    TokenEndpoint         string `json:"token_endpoint"`
    Issuer                string `json:"issuer"`
}
```

**PKCE Data:**
```go
type PKCEData struct {
    CodeVerifier  string // 43-128 chars, base64url-encoded
    CodeChallenge string // SHA256(code_verifier), base64url-encoded
}
```

**Token Response:**
```go
type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token,omitempty"`
    ExpiresIn    int    `json:"expires_in"`
    TokenType    string `json:"token_type"`
}
```

## Functional Requirements

### FR-002: OAuth2.1 Discovery and Authentication
- **Description:** Discover OAuth2.1 endpoints via .well-known mechanism and execute PKCE-enhanced authorization code flow.
- **Inputs:**
  - MCP server URL. Example: `https://mcp.example.com`
  - `client_id`. Example: `123456789.apps.googleusercontent.com`
  - `client_secret`. Example: `GOCSPX-abc123...`
  - OAuth2.1 authorization response (authorization code from callback). Example: `4/0AY0e-g7X...`
- **Outputs:**
  - `access_token`. Example: `ya29.a0AfH6SMBx...`
  - `refresh_token`. Example: `1//0gHZ9K...`
  - `expires_in`. Example: `3600` (seconds)
- **Business Rules:**
  - Fetch `.well-known/oauth-authorization-server` from MCP server URL
  - Parse `authorization_endpoint` and `token_endpoint` from discovery document
  - Generate cryptographically secure PKCE `code_verifier` (43-128 characters, base64url-encoded)
  - Compute `code_challenge` as SHA256(code_verifier), base64url-encoded
  - Start temporary HTTP server on localhost:3000 (or next available port 3001-3010)
  - Construct authorization URL with: `client_id`, `redirect_uri` (http://localhost:{port}/oauth2callback), `response_type=code`, `code_challenge`, `code_challenge_method=S256`, `scope` (if required by server)
  - Open system default browser to authorization URL
  - Wait for callback with authorization code (timeout: 5 minutes)
  - Exchange authorization code for tokens using: `client_id`, `client_secret`, `code`, `redirect_uri`, `code_verifier`, `grant_type=authorization_code`
  - Stop temporary HTTP server after receiving callback
  - Validate token response contains `access_token` and `expires_in`
  - `refresh_token` is optional but should be stored if provided

## Acceptance Tests

> **Acceptance tests are mandatory: 100% must pass.**
> Tests MUST be validated through `go test ./...` or `make test`.

### Test Data

| Data | Description | Source | Status |
|------|-------------|--------|--------|
| Mock OAuth server | HTTP server returning .well-known and token responses | auto-generated with httptest | ready |
| Mock MCP server | HTTP server for discovery endpoint | auto-generated with httptest | ready |
| Test credentials | client_id, client_secret for testing | auto-generated in test setup | ready |
| Authorization codes | Valid and invalid auth codes | auto-generated in test cases | ready |

### Happy Path Tests

#### E2E-001: First-time OAuth flow with token caching
- **Category:** happy path
- **Scenario:** SC-001 — First-time use (no cached token)
- **Requirements:** FR-001, FR-002, FR-003, FR-005
- **Preconditions:**
  - No token file exists at ~/.cache/mcp-proxy/{url_base64}.json
  - Environment variables: GOOGLE_CLIENT_ID="test-client-id", GOOGLE_CLIENT_SECRET="test-secret"
  - Mock MCP server running at https://mcp.test.com
  - Mock OAuth server configured with .well-known discovery
- **Steps:**
  - Given no cached token exists for https://mcp.test.com
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - Then mcp-proxy validates credentials are not empty
  - And mcp-proxy discovers OAuth endpoints via .well-known/oauth-authorization-server
  - And mcp-proxy starts HTTP server on localhost:3000
  - And mcp-proxy opens browser to authorization URL with PKCE challenge
  - And user completes OAuth flow (simulated in test)
  - And mcp-proxy exchanges auth code for tokens with PKCE verifier
  - And token file is created at ~/.cache/mcp-proxy/{url_base64}.json with permissions 0600
  - And token file contains: access_token, refresh_token, expiration_time (ISO 8601 UTC)
  - And MCP server responds with status 200 (simulated)
  - And response is forwarded to stdout (simulated)
- **Cleanup:** Delete token file, stop mock servers
- **Priority:** Critical

### Edge Case and Error Tests

#### E2E-008: User cancels OAuth flow (timeout)
- **Category:** error
- **Scenario:** SC-001
- **Requirements:** FR-002, FR-006
- **Preconditions:**
  - Valid credentials, no cached token
  - Mock OAuth server running
- **Steps:**
  - Given no cached token exists
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - And mcp-proxy opens browser for OAuth
  - And user does not complete OAuth flow (simulated by not sending callback)
  - And 5 minutes elapse
  - Then mcp-proxy exits with code 2
  - And stderr contains: "Error: OAuth flow timed out. User did not complete authentication."
  - And HTTP callback server is stopped
  - And no token file is created
- **Cleanup:** None
- **Priority:** High

#### E2E-013: OAuth token exchange fails
- **Category:** error
- **Scenario:** SC-001, SC-004
- **Requirements:** FR-002, FR-006
- **Preconditions:**
  - No cached token
  - Mock OAuth server configured to reject token exchange
- **Steps:**
  - Given no cached token exists
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - And OAuth flow completes with authorization code
  - And mcp-proxy attempts token exchange
  - And OAuth server responds with status 400 (invalid_client)
  - Then mcp-proxy exits with code 2
  - And stderr contains: "Error: Failed to exchange authorization code for tokens: invalid_client"
  - And no token file is created
- **Cleanup:** None
- **Priority:** High

### Side Effect Tests

#### E2E-019: Temporary HTTP server lifecycle
- **Category:** side effect
- **Scenario:** SC-001
- **Requirements:** FR-002
- **Preconditions:**
  - No cached token
- **Steps:**
  - Given no cached token exists
  - When mcp-proxy starts OAuth flow
  - Then HTTP server starts on localhost:3000 (or next available port)
  - And server only listens on 127.0.0.1 (not 0.0.0.0)
  - And server accepts callback at /oauth2callback
  - When callback is received
  - Then HTTP server stops immediately
  - And port is released
- **Cleanup:** None
- **Priority:** High

#### E2E-021: Concurrent mcp-proxy instances with different URLs
- **Category:** edge case
- **Scenario:** SC-001
- **Requirements:** FR-002
- **Preconditions:**
  - No cached tokens
  - Two different MCP server URLs
- **Steps:**
  - Given no cached tokens exist
  - When user runs: `mcp-proxy -u https://mcp1.test.com` in terminal 1
  - And user runs: `mcp-proxy -u https://mcp2.test.com` in terminal 2
  - Then both instances start OAuth flows independently
  - And instance 1 uses port 3000, instance 2 uses port 3001
  - And both complete successfully
  - And two separate token files are created with different filenames
  - And no conflicts occur
- **Cleanup:** Delete both token files
- **Priority:** High

#### E2E-027: HTTPS enforcement (reject HTTP URLs)
- **Category:** security
- **Scenario:** All
- **Requirements:** FR-001, FR-002
- **Preconditions:**
  - Valid credentials
- **Steps:**
  - Given GOOGLE_CLIENT_ID="test-client-id", GOOGLE_CLIENT_SECRET="test-secret"
  - When user runs: `mcp-proxy -u http://mcp.test.com`
  - Then mcp-proxy exits with code 1
  - And stderr contains: "Error: MCP server URL must use HTTPS"
  - And no network connection is attempted
- **Cleanup:** None
- **Priority:** Critical

## Constraints

### Files Not to Touch
- `internal/config/config.go` (from US-001, only read)
- `internal/token/storage.go` (from US-001, only write tokens)
- `internal/errors/errors.go` (from US-001, only use)

### Dependencies Not to Add
- Only standard library packages allowed
- No external OAuth libraries (implement from scratch)
- No external HTTP client libraries (use `net/http`)

### Patterns to Avoid
- Do not store PKCE code_verifier in global variables
- Do not log authorization codes or tokens
- Do not use HTTP (only HTTPS for OAuth endpoints)
- Do not keep HTTP server running after callback

### Scope Boundary
- **NOT in this story:** Token refresh logic, MCP server proxy, stdio interface
- **IN this story:** OAuth discovery, PKCE generation, callback server, browser integration, token exchange

## Non Regression

### Existing Tests That Must Pass
- All tests from US-001 (CLI parsing, token file I/O, error handling)

### Behaviors That Must Not Change
- Configuration parsing and validation (from US-001)
- Token file format and permissions (from US-001)
- Error message format and exit codes (from US-001)

### API Contracts to Preserve
- `Config` struct from US-001
- `TokenData` struct from US-001
- Error types and exit codes from US-001
