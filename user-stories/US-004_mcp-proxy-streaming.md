# US-004: MCP Server Proxy & Streaming

> Parent Spec: specs/2026-04-15_22:30:35-mcp-proxy.md
> Status: ready
> Priority: 4
> Depends On: US-001, US-003
> Complexity: M

## Objective

Implement the core MCP proxy functionality: expose a stdio interface for local MCP clients, forward JSON-RPC messages to remote MCP servers via streamable HTTP with OAuth2.1 authentication, handle 401 responses with automatic token refresh, and integrate OpenTelemetry tracing. This story completes the end-to-end user experience, enabling developers to use OAuth2.1-protected MCP servers transparently.

## Technical Context

### Stack
- **Language:** Go 1.21+
- **HTTP Client:** Standard library `net/http` with streaming support
- **I/O:** Standard library `io`, `bufio` for stdin/stdout
- **JSON:** Standard library `encoding/json`
- **OpenTelemetry:** `go.opentelemetry.io/otel` for tracing
- **Testing:** Go testing framework with mock stdin/stdout and HTTP servers

### Relevant File Structure
```
mcp-proxy/
├── main.go                    # Entry point, stdio loop
├── internal/
│   ├── proxy/
│   │   ├── stdio.go          # stdio interface
│   │   ├── http.go           # HTTP client for MCP server
│   │   └── handler.go        # Request/response handling
│   ├── telemetry/
│   │   ├── tracer.go         # OpenTelemetry setup
│   │   └── exporter.go       # JSONL file exporter
│   ├── token/
│   │   └── refresh.go        # (from US-003, reuse)
│   └── config/
│       └── config.go         # (from US-001)
├── tests/
│   ├── proxy_stdio_test.go
│   ├── proxy_http_test.go
│   └── telemetry_test.go
└── go.mod
```

### Existing Patterns
From US-001:
- Configuration object
- Token file I/O
- Error handling with exit codes

From US-002:
- HTTP client with timeouts
- OAuth token exchange

From US-003:
- Token refresh logic
- Expiration detection

New patterns to establish:
- **stdio interface:** Read newline-delimited JSON from stdin, write to stdout
- **Streaming HTTP:** Use `http.Client` with streaming response bodies
- **401 handling:** Detect 401, refresh token, retry request once
- **OpenTelemetry:** JSONL file exporter at `~/.cache/mcp-proxy/traces.jsonl`
- **Graceful shutdown:** Handle SIGINT, close connections cleanly

### Data Model (excerpt)

**JSON-RPC Message:**
```go
type JSONRPCMessage struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id,omitempty"`
    Method  string          `json:"method,omitempty"`
    Params  json.RawMessage `json:"params,omitempty"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *JSONRPCError   `json:"error,omitempty"`
}
```

**Trace Span Attributes:**
```go
type SpanAttributes struct {
    MCPServerURL      string // Sanitized URL
    OAuthFlowStep     string // "discovery", "authorization", "token_exchange", "refresh"
    HTTPMethod        string
    HTTPStatusCode    int
    HTTPDurationMs    int64
    ErrorType         string
    ErrorMessage      string // Sanitized
    TokenExpiration   string // ISO 8601
    TokenRefreshAttempted bool
}
```

## Functional Requirements

### FR-005: MCP Server Proxy
- **Description:** Expose stdio interface locally and proxy requests/responses to remote MCP server via streamable HTTP with OAuth2.1 authentication.
- **Inputs:**
  - stdin from local MCP client. Example:
    ```json
    {"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}
    ```
  - MCP server URL. Example: `https://mcp.example.com`
  - Valid `access_token`. Example: `ya29.a0AfH6SMBx...`
- **Outputs:**
  - stdout to local MCP client (proxied responses from MCP server). Example:
    ```json
    {"jsonrpc":"2.0","id":1,"result":{"tools":[...]}}
    ```
- **Business Rules:**
  - Read JSON-RPC messages from stdin (newline-delimited)
  - Forward each message to MCP server via streamable HTTP POST request
  - Include Authorization header: "Bearer {access_token}"
  - Include Content-Type header: "application/json"
  - Stream response from MCP server back to stdout
  - Preserve message ordering (FIFO)
  - If MCP server returns 401 Unauthorized, treat as token expired and attempt refresh (FR-004)
  - If refresh succeeds, retry the failed request automatically
  - If refresh fails, exit with error message
  - Handle connection errors gracefully with clear error messages
  - Request timeout: 30 seconds per request

## Acceptance Tests

> **Acceptance tests are mandatory: 100% must pass.**
> Tests MUST be validated through `go test ./...` or `make test`.

### Test Data

| Data | Description | Source | Status |
|------|-------------|--------|--------|
| Mock MCP server | HTTP server returning JSON-RPC responses | auto-generated with httptest | ready |
| Mock stdin | Simulated JSON-RPC requests | auto-generated in test cases | ready |
| Mock stdout | Captured output for verification | auto-generated in test cases | ready |
| Valid tokens | access_token for Authorization header | auto-generated in test setup | ready |

### Happy Path Tests

#### E2E-002: Subsequent use with valid cached token
- **Category:** happy path
- **Scenario:** SC-002 — Subsequent use with valid token
- **Requirements:** FR-001, FR-003, FR-005
- **Preconditions:**
  - Token file exists with valid access_token (expiration_time > current_time + 5 minutes)
  - Mock MCP server running
- **Steps:**
  - Given token file exists with access_token="valid-token", expiration_time="2026-04-15T23:00:00Z"
  - And current time is 2026-04-15T22:00:00Z
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - Then mcp-proxy reads token file
  - And mcp-proxy validates expiration_time > current_time
  - And mcp-proxy connects to MCP server with Authorization: Bearer valid-token
  - And MCP server responds with status 200
  - And response is forwarded to stdout
  - And no browser interaction occurs
  - And token file is unchanged
- **Cleanup:** Delete token file
- **Priority:** Critical

### Edge Case and Error Tests

#### E2E-009: MCP server rejects token (revoked server-side)
- **Category:** error
- **Scenario:** SC-002
- **Requirements:** FR-004, FR-006
- **Preconditions:**
  - Token file exists with valid-looking token
  - Mock MCP server configured to reject token with 401
  - Mock OAuth server configured to accept refresh
- **Steps:**
  - Given token file exists with access_token="revoked-token", refresh_token="valid-refresh", expiration_time="2026-04-15T23:00:00Z"
  - And current time is 2026-04-15T22:00:00Z (token not expired)
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - And mcp-proxy connects to MCP server with Authorization: Bearer revoked-token
  - And MCP server responds with status 401
  - Then mcp-proxy attempts token refresh
  - And OAuth server provides new tokens
  - And mcp-proxy retries MCP request with new token
  - And MCP server responds with status 200
  - And response is forwarded to stdout
- **Cleanup:** Delete token file
- **Priority:** Critical

#### E2E-010: Network error connecting to MCP server
- **Category:** error
- **Scenario:** SC-002
- **Requirements:** FR-005, FR-006
- **Preconditions:**
  - Token file exists with valid token
  - MCP server is unreachable (connection refused)
- **Steps:**
  - Given token file exists with valid token
  - And MCP server at https://mcp.test.com is not running
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - Then mcp-proxy attempts to connect to MCP server
  - And connection fails (connection refused)
  - Then mcp-proxy exits with code 3
  - And stderr contains: "Error: Failed to connect to MCP server: connection refused"
- **Cleanup:** Delete token file
- **Priority:** High

#### E2E-022: Corrupted token file (invalid JSON)
- **Category:** edge case
- **Scenario:** SC-002
- **Requirements:** FR-003
- **Preconditions:**
  - Token file exists but contains invalid JSON
- **Steps:**
  - Given token file exists with content: "{invalid json"
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - Then mcp-proxy treats file as missing
  - And proceeds with full OAuth flow
  - And new valid token file is created
  - And operation succeeds
- **Cleanup:** Delete token file
- **Priority:** Medium

#### E2E-025: Very long MCP server response (streaming)
- **Category:** edge case
- **Scenario:** All
- **Requirements:** FR-005
- **Preconditions:**
  - Valid token
  - Mock MCP server configured to send large response (10MB)
- **Steps:**
  - Given valid token exists
  - When MCP client sends request
  - And MCP server responds with 10MB streaming response
  - Then mcp-proxy streams response to stdout without buffering entire response
  - And memory usage remains < 50MB
  - And response is complete and correct
- **Cleanup:** Delete token file
- **Priority:** Low

#### E2E-026: Graceful handling of SIGINT (Ctrl+C)
- **Category:** edge case
- **Scenario:** All
- **Requirements:** FR-006
- **Preconditions:**
  - mcp-proxy is running
- **Steps:**
  - Given mcp-proxy is running and waiting for MCP requests
  - When user sends SIGINT (Ctrl+C)
  - Then mcp-proxy stops HTTP server (if running)
  - And closes MCP server connection gracefully
  - And exits with code 0
  - And no error message is printed
- **Cleanup:** None
- **Priority:** Low

### Side Effect Tests

#### E2E-020: OpenTelemetry trace file created
- **Category:** side effect
- **Scenario:** All
- **Requirements:** FR-005
- **Preconditions:**
  - ~/.cache/mcp-proxy/ directory exists
- **Steps:**
  - Given any mcp-proxy operation
  - When mcp-proxy runs
  - Then trace file is created at ~/.cache/mcp-proxy/traces.jsonl
  - And file permissions are 0600
  - And file contains JSONL-formatted trace entries
  - And trace entries include: oauth.flow.step, http.method, http.status_code
  - And trace entries do NOT include: access_token, refresh_token, client_secret, code_verifier
- **Cleanup:** Delete trace file
- **Priority:** High

## Constraints

### Files Not to Touch
- `internal/config/config.go` (from US-001, only read)
- `internal/token/storage.go` (from US-001, only read/write)
- `internal/token/refresh.go` (from US-003, reuse)
- `internal/oauth/` (from US-002, reuse for fallback)

### Dependencies Not to Add
- OpenTelemetry SDK: `go.opentelemetry.io/otel` and `go.opentelemetry.io/otel/sdk`
- No external JSON-RPC libraries (implement from scratch)
- No external streaming libraries (use `net/http`)

### Patterns to Avoid
- Do not buffer entire response in memory (stream it)
- Do not log request/response bodies (only metadata)
- Do not retry requests indefinitely (max 1 retry after refresh)
- Do not block on stdin reads (use goroutines if needed)

### Scope Boundary
- **NOT in this story:** Additional OAuth providers, GUI, multi-server support
- **IN this story:** stdio interface, HTTP proxy, 401 handling, OpenTelemetry tracing, graceful shutdown

## Non Regression

### Existing Tests That Must Pass
- All tests from US-001 (CLI parsing, token file I/O, error handling)
- All tests from US-002 (OAuth discovery, PKCE, callback server, token exchange)
- All tests from US-003 (token refresh, expiration detection, atomic updates)

### Behaviors That Must Not Change
- Configuration parsing and validation (from US-001)
- Token file format and permissions (from US-001)
- Error message format and exit codes (from US-001)
- OAuth discovery and PKCE generation (from US-002)
- Token exchange flow (from US-002)
- Token refresh logic (from US-003)
- Expiration detection (from US-003)

### API Contracts to Preserve
- `Config` struct from US-001
- `TokenData` struct from US-001
- `OAuthDiscovery` struct from US-002
- `TokenResponse` struct from US-002
- Error types and exit codes from US-001
- Token refresh interface from US-003
