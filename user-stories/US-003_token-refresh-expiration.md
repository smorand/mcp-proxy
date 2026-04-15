# US-003: Token Refresh & Expiration

> Parent Spec: specs/2026-04-15_22:30:35-mcp-proxy.md
> Status: done
> Priority: 3
> Depends On: US-001, US-002
> Complexity: M

## Objective

Implement automatic token expiration detection and refresh logic using refresh_token. This story enables mcp-proxy to seamlessly refresh expired access tokens without user interaction, and gracefully fall back to full OAuth re-authentication when refresh tokens are invalid or expired. This provides a smooth user experience by minimizing authentication interruptions.

## Technical Context

### Stack
- **Language:** Go 1.21+
- **HTTP Client:** Standard library `net/http`
- **Time:** Standard library `time`
- **Testing:** Go testing framework with mock HTTP servers and time manipulation

### Relevant File Structure
```
mcp-proxy/
├── internal/
│   ├── token/
│   │   ├── storage.go        # (from US-001)
│   │   ├── refresh.go        # Token refresh logic
│   │   └── expiration.go     # Expiration detection
│   ├── oauth/
│   │   └── exchange.go       # (from US-002, reuse for refresh)
│   └── config/
│       └── config.go         # (from US-001)
├── tests/
│   ├── token_refresh_test.go
│   └── token_expiration_test.go
└── go.mod
```

### Existing Patterns
From US-001:
- Token file I/O with atomic writes
- Error handling with specific exit codes
- Configuration object

From US-002:
- OAuth token exchange (reuse for refresh)
- HTTP client with timeouts
- Mock HTTP servers for testing

New patterns to establish:
- **Expiration detection:** Compare current time with `expiration_time` from token file
- **Refresh flow:** POST to token endpoint with `grant_type=refresh_token`
- **Fallback logic:** If refresh fails with 400/401, trigger full OAuth flow from US-002
- **Atomic updates:** Write new tokens to temp file, then rename

### Data Model (excerpt)

**Token Refresh Request:**
```go
type RefreshRequest struct {
    ClientID     string `json:"client_id"`
    ClientSecret string `json:"client_secret"`
    RefreshToken string `json:"refresh_token"`
    GrantType    string `json:"grant_type"` // "refresh_token"
}
```

**Token Refresh Response:**
```go
type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token,omitempty"` // New refresh token (optional)
    ExpiresIn    int    `json:"expires_in"`
    TokenType    string `json:"token_type"`
}
```

## Functional Requirements

### FR-004: Token Expiration and Refresh
- **Description:** Check token expiration before use and automatically refresh expired tokens using refresh_token.
- **Inputs:**
  - Token file contents. Example:
    ```json
    {
      "access_token": "ya29.a0AfH6SMBx...",
      "refresh_token": "1//0gHZ9K...",
      "expiration_time": "2026-04-15T21:00:00Z"
    }
    ```
  - Current system time. Example: `2026-04-15T22:00:00Z`
- **Outputs:**
  - Valid `access_token` (either existing or refreshed). Example: `ya29.a0AfH6SMBx...` (new)
  - Updated token file (if refresh occurred)
- **Business Rules:**
  - Compare current system time with `expiration_time` from token file
  - If `current_time < expiration_time`, use existing `access_token`
  - If `current_time >= expiration_time`, attempt token refresh:
    - Use `refresh_token` to request new `access_token` from OAuth2.1 token endpoint
    - Request parameters: `client_id`, `client_secret`, `refresh_token`, `grant_type=refresh_token`
    - If refresh succeeds: update token file with new `access_token`, new `expiration_time`, and new `refresh_token` (if provided)
    - If refresh fails (401/400 response): proceed with full OAuth flow (SC-004)
  - If `refresh_token` is missing from token file, proceed with full OAuth flow
  - Token refresh should complete within 2 seconds (network timeout)

## Acceptance Tests

> **Acceptance tests are mandatory: 100% must pass.**
> Tests MUST be validated through `go test ./...` or `make test`.

### Test Data

| Data | Description | Source | Status |
|------|-------------|--------|--------|
| Mock OAuth server | HTTP server for token refresh endpoint | auto-generated with httptest | ready |
| Expired tokens | Token files with past expiration_time | auto-generated in test setup | ready |
| Valid refresh tokens | refresh_token values accepted by mock server | auto-generated in test cases | ready |
| Invalid refresh tokens | refresh_token values rejected by mock server | auto-generated in test cases | ready |

### Happy Path Tests

#### E2E-003: Automatic token refresh with valid refresh_token
- **Category:** happy path
- **Scenario:** SC-003 — Token expired, refresh available
- **Requirements:** FR-001, FR-003, FR-004, FR-005
- **Preconditions:**
  - Token file exists with expired access_token and valid refresh_token
  - Mock OAuth server configured to accept refresh_token
- **Steps:**
  - Given token file exists with access_token="expired-token", refresh_token="valid-refresh", expiration_time="2026-04-15T21:00:00Z"
  - And current time is 2026-04-15T22:00:00Z
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - Then mcp-proxy reads token file
  - And mcp-proxy detects expiration_time < current_time
  - And mcp-proxy sends refresh request to OAuth server with refresh_token="valid-refresh"
  - And OAuth server responds with new access_token="new-token", refresh_token="new-refresh", expires_in=3600
  - And token file is updated with access_token="new-token", refresh_token="new-refresh", expiration_time="2026-04-15T23:00:00Z"
  - And mcp-proxy connects to MCP server with Authorization: Bearer new-token (simulated)
  - And MCP server responds with status 200 (simulated)
  - And response is forwarded to stdout (simulated)
  - And no browser interaction occurs
- **Cleanup:** Delete token file
- **Priority:** Critical

#### E2E-004: Re-authentication when refresh_token is invalid
- **Category:** happy path
- **Scenario:** SC-004 — Token expired, no valid refresh
- **Requirements:** FR-001, FR-002, FR-003, FR-004, FR-005
- **Preconditions:**
  - Token file exists with expired access_token and invalid refresh_token
  - Mock OAuth server configured to reject refresh_token
- **Steps:**
  - Given token file exists with access_token="expired-token", refresh_token="invalid-refresh", expiration_time="2026-04-15T21:00:00Z"
  - And current time is 2026-04-15T22:00:00Z
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - Then mcp-proxy reads token file
  - And mcp-proxy detects expiration_time < current_time
  - And mcp-proxy sends refresh request to OAuth server
  - And OAuth server responds with status 400 (invalid_grant)
  - And mcp-proxy starts full OAuth flow (discovery, browser, token exchange)
  - And new tokens are obtained and saved
  - And mcp-proxy connects to MCP server with new access_token (simulated)
  - And MCP server responds with status 200 (simulated)
  - And response is forwarded to stdout (simulated)
- **Cleanup:** Delete token file
- **Priority:** Critical

### Edge Case and Error Tests

#### E2E-011: Refresh token rejected by OAuth server
- **Category:** error
- **Scenario:** SC-003
- **Requirements:** FR-004, FR-006
- **Preconditions:**
  - Token file exists with expired access_token
  - Mock OAuth server configured to reject refresh_token
  - Mock OAuth server configured for full OAuth flow
- **Steps:**
  - Given token file exists with expired access_token and refresh_token
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - And mcp-proxy attempts token refresh
  - And OAuth server responds with status 400 (invalid_grant)
  - Then mcp-proxy falls back to full OAuth flow
  - And browser is opened for re-authentication
  - And new tokens are obtained
  - And MCP request succeeds (simulated)
- **Cleanup:** Delete token file
- **Priority:** Critical

#### E2E-012: Network error during token refresh
- **Category:** error
- **Scenario:** SC-003
- **Requirements:** FR-004, FR-006
- **Preconditions:**
  - Token file exists with expired token
  - OAuth server is unreachable
- **Steps:**
  - Given token file exists with expired access_token
  - And OAuth server is not reachable
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - And mcp-proxy attempts token refresh
  - And connection to OAuth server fails
  - Then mcp-proxy exits with code 3
  - And stderr contains: "Error: Failed to refresh token: connection refused"
- **Cleanup:** Delete token file
- **Priority:** High

### Side Effect Tests

#### E2E-018: Token file updated after refresh
- **Category:** side effect
- **Scenario:** SC-003
- **Requirements:** FR-003, FR-004
- **Preconditions:**
  - Token file exists with expired token
  - Refresh will succeed
- **Steps:**
  - Given token file exists with old_access_token, old_refresh_token, old_expiration
  - When token refresh succeeds
  - Then token file is updated with new_access_token, new_refresh_token, new_expiration
  - And file permissions remain 0600
  - And old token values are completely replaced
  - And expiration_time is correctly computed as current_time + expires_in
- **Cleanup:** Delete token file
- **Priority:** Critical

#### E2E-024: Token expires during long-running session
- **Category:** edge case
- **Scenario:** All
- **Requirements:** FR-004
- **Preconditions:**
  - Token file exists with token expiring soon
  - MCP client sends multiple requests
- **Steps:**
  - Given token file with expiration_time in 10 seconds
  - When first MCP request succeeds with current token (simulated)
  - And 15 seconds elapse
  - And second MCP request is sent
  - Then mcp-proxy detects token expiration
  - And automatically refreshes token
  - And second request succeeds with new token
  - And no user interaction required
- **Cleanup:** Delete token file
- **Priority:** Medium

#### E2E-030: Atomic token file writes
- **Category:** data integrity
- **Scenario:** SC-003
- **Requirements:** FR-003, FR-004
- **Preconditions:**
  - Token file exists
  - Refresh will succeed
- **Steps:**
  - Given token file exists with old tokens
  - When token refresh completes
  - Then new tokens are written to temporary file first
  - And temporary file is renamed to final filename (atomic operation)
  - And if process is killed during write, either old or new file exists (never corrupted)
  - And file is never in partially-written state
- **Cleanup:** Delete token file
- **Priority:** High

## Constraints

### Files Not to Touch
- `internal/config/config.go` (from US-001, only read)
- `internal/oauth/discovery.go` (from US-002, only read)
- `internal/oauth/pkce.go` (from US-002, not needed)
- `internal/oauth/callback.go` (from US-002, reuse for fallback)
- `internal/oauth/browser.go` (from US-002, reuse for fallback)

### Dependencies Not to Add
- Only standard library packages allowed
- No external time manipulation libraries (use `time` package)
- No external HTTP client libraries (use `net/http`)

### Patterns to Avoid
- Do not cache tokens in memory across invocations
- Do not log refresh tokens
- Do not retry refresh indefinitely (max 1 attempt, then fallback)
- Do not use non-atomic file writes

### Scope Boundary
- **NOT in this story:** MCP server proxy, stdio interface, OpenTelemetry tracing
- **IN this story:** Token expiration detection, refresh logic, fallback to full OAuth, atomic token updates

## Non Regression

### Existing Tests That Must Pass
- All tests from US-001 (CLI parsing, token file I/O, error handling)
- All tests from US-002 (OAuth discovery, PKCE, callback server, token exchange)

### Behaviors That Must Not Change
- Configuration parsing and validation (from US-001)
- Token file format and permissions (from US-001)
- Error message format and exit codes (from US-001)
- OAuth discovery and PKCE generation (from US-002)
- Token exchange flow (from US-002)

### API Contracts to Preserve
- `Config` struct from US-001
- `TokenData` struct from US-001
- `OAuthDiscovery` struct from US-002
- `TokenResponse` struct from US-002
- Error types and exit codes from US-001
