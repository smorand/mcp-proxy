# US-001: Foundation: CLI & Token File Management

> Parent Spec: specs/2026-04-15_22:30:35-mcp-proxy.md
> Status: ready
> Priority: 1
> Depends On: none
> Complexity: M

## Objective

Establish the foundational infrastructure for mcp-proxy: CLI argument parsing with environment variable support, secure token file management with proper permissions, and a robust error handling framework. This story provides the base layer that all subsequent OAuth and proxy functionality will build upon.

## Technical Context

### Stack
- **Language:** Go 1.21+
- **CLI Framework:** Standard library `flag` package or `spf13/cobra` (to be decided)
- **File I/O:** Standard library `os`, `io/ioutil`
- **JSON:** Standard library `encoding/json`
- **Testing:** Go testing framework with table-driven tests

### Relevant File Structure
```
mcp-proxy/
├── main.go                    # Entry point, CLI setup
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration parsing and validation
│   ├── token/
│   │   └── storage.go        # Token file I/O operations
│   └── errors/
│       └── errors.go         # Error types and exit codes
├── tests/
│   ├── config_test.go
│   ├── token_storage_test.go
│   └── errors_test.go
└── go.mod
```

### Existing Patterns
This is a greenfield project. Patterns to establish:
- **Error handling:** All errors must start with "Error: " and use specific exit codes (1-5)
- **Configuration:** Support both CLI flags and environment variables with "env:" prefix
- **File operations:** Atomic writes (temp file + rename), strict permission enforcement
- **Testing:** Table-driven tests with comprehensive edge case coverage

### Data Model (excerpt)

**Configuration Object:**
```go
type Config struct {
    ServerURL    string // MCP server URL (required, must be HTTPS)
    ClientID     string // OAuth2.1 client ID (resolved from flag or env)
    ClientSecret string // OAuth2.1 client secret (resolved from flag or env)
}
```

**Token File Structure:**
```go
type TokenData struct {
    AccessToken    string    `json:"access_token"`
    RefreshToken   string    `json:"refresh_token,omitempty"`
    ExpirationTime time.Time `json:"expiration_time"`
}
```

**Token File Location:** `~/.cache/mcp-proxy/{base64url(server_url)}.json`

## Functional Requirements

### FR-001: CLI Argument Parsing
- **Description:** Parse and validate command-line arguments for mcp-proxy configuration.
- **Inputs:**
  - `--url` or `-u`: MCP server URL (required). Example: `https://mcp.example.com`
  - `--client-id` or `-i`: OAuth2.1 client ID (optional, default: `env:GOOGLE_CLIENT_ID`). Example: `my-client-id` or `env:MY_CLIENT_ID`
  - `--client-secret` or `-s`: OAuth2.1 client secret (optional, default: `env:GOOGLE_CLIENT_SECRET`). Example: `my-secret` or `env:MY_SECRET`
- **Outputs:**
  - Validated `Config` object with resolved `client_id`, `client_secret`, and `server_url`
- **Business Rules:**
  - URL flag is mandatory; exit with usage help if missing
  - If `client_id` starts with "env:", read value from environment variable (e.g., "env:GOOGLE_CLIENT_ID" reads `$GOOGLE_CLIENT_ID`)
  - If `client_secret` starts with "env:", read value from environment variable
  - Default `client_id` is "env:GOOGLE_CLIENT_ID"
  - Default `client_secret` is "env:GOOGLE_CLIENT_SECRET"
  - If resolved `client_id` is empty, exit with error: "Error: client_id is required. Set via --client-id flag or GOOGLE_CLIENT_ID environment variable." (exit code 1)
  - If resolved `client_secret` is empty, exit with error: "Error: client_secret is required. Set via --client-secret flag or GOOGLE_CLIENT_SECRET environment variable." (exit code 1)
  - URL must start with "https://"; reject "http://" URLs with error: "Error: MCP server URL must use HTTPS. Insecure HTTP connections are not allowed." (exit code 1)
  - URL must be a valid URL format; reject malformed URLs with error: "Error: Invalid MCP server URL format" (exit code 1)

### FR-003: Token File Management
- **Description:** Persist OAuth2.1 tokens to local filesystem with secure permissions and always read from disk (no in-memory caching).
- **Inputs:**
  - MCP server URL (for filename generation). Example: `https://mcp.example.com`
  - `access_token`: Example: `ya29.a0AfH6SMBx...`
  - `refresh_token`: Example: `1//0gHZ9K...` (optional)
  - `expires_in`: Example: `3600` (seconds)
- **Outputs:**
  - Token file at `~/.cache/mcp-proxy/{url_base64}.json`
- **Business Rules:**
  - Create `~/.cache/mcp-proxy/` directory if it doesn't exist (permissions: 0700)
  - Filename is base64url-encoded MCP server URL with `.json` extension
  - Token file format (JSON):
    ```json
    {
      "access_token": "ya29.a0AfH6SMBx...",
      "refresh_token": "1//0gHZ9K...",
      "expiration_time": "2026-04-15T22:45:00Z"
    }
    ```
  - `expiration_time` is computed as: `current_time + expires_in` (ISO 8601 UTC format)
  - Token file permissions must be 0600 (read/write for owner only)
  - Always read token file from disk on each mcp-proxy invocation (no in-memory caching across invocations)
  - If token file read fails, treat as missing token and return error
  - If token file is invalid JSON, treat as missing token and return error
  - If token file write fails, exit with error: "Error: Cannot save tokens to ~/.cache/mcp-proxy/{url_base64}.json: {error details}" (exit code 4)
  - Token file writes must be atomic: write to temp file, then rename

### FR-006: Error Handling and User Feedback
- **Description:** Provide clear, actionable error messages for all failure scenarios.
- **Inputs:**
  - Error conditions from any component (CLI parsing, token management)
- **Outputs:**
  - Human-readable error messages to stderr
  - Non-zero exit codes
- **Business Rules:**
  - All error messages must start with "Error: "
  - Error messages must be specific and actionable (tell user what went wrong and how to fix it)
  - Never include sensitive data in error messages (tokens, secrets)
  - Exit codes:
    - 0: Success
    - 1: Configuration error (missing/invalid arguments)
    - 2: Authentication error (OAuth flow failed)
    - 3: Network error (cannot reach MCP server or OAuth server)
    - 4: File system error (cannot read/write token file)
    - 5: Token error (token expired and refresh failed)
  - For file system errors, include specific error details (permission denied, disk full, etc.)

## Acceptance Tests

> **Acceptance tests are mandatory: 100% must pass.**
> A user story is NOT considered implemented until **every single acceptance test below passes**.
> The implementing agent MUST loop (fix code → run tests → check results → repeat) until all acceptance tests pass with zero failures. Do not stop or declare the story "done" while any test is failing.
> Tests MUST be validated through the project's test command (e.g., `go test ./...` or `make test`). No other method of running or validating tests is acceptable.

### Test Data

All test data is auto-generated within test code. No external fixtures required.

| Data | Description | Source | Status |
|------|-------------|--------|--------|
| Mock environment variables | GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET for testing | auto-generated in test setup | ready |
| Test URLs | Valid and invalid HTTPS/HTTP URLs | auto-generated in test cases | ready |
| Token file content | Valid and invalid JSON token data | auto-generated in test cases | ready |
| Temporary directories | Isolated test directories for token storage | auto-generated per test | ready |

### Happy Path Tests

None in this story. This story focuses on validation and error handling. Happy paths are tested in subsequent stories when OAuth flow is implemented.

### Edge Case and Error Tests

#### E2E-005: Empty client_id causes clear error
- **Category:** error
- **Scenario:** SC-001
- **Requirements:** FR-001, FR-006
- **Preconditions:**
  - GOOGLE_CLIENT_ID environment variable is empty or not set
- **Steps:**
  - Given GOOGLE_CLIENT_ID="" (empty)
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - Then mcp-proxy exits with code 1
  - And stderr contains: "Error: client_id is required. Set via --client-id flag or GOOGLE_CLIENT_ID environment variable."
  - And no token file is created
  - And no HTTP server is started
- **Cleanup:** None
- **Priority:** Critical

#### E2E-006: Empty client_secret causes clear error
- **Category:** error
- **Scenario:** SC-001
- **Requirements:** FR-001, FR-006
- **Preconditions:**
  - GOOGLE_CLIENT_SECRET environment variable is empty or not set
- **Steps:**
  - Given GOOGLE_CLIENT_SECRET="" (empty)
  - And GOOGLE_CLIENT_ID="test-client-id"
  - When user runs: `mcp-proxy -u https://mcp.test.com`
  - Then mcp-proxy exits with code 1
  - And stderr contains: "Error: client_secret is required. Set via --client-secret flag or GOOGLE_CLIENT_SECRET environment variable."
  - And no token file is created
  - And no HTTP server is started
- **Cleanup:** None
- **Priority:** Critical

#### E2E-007: HTTP URL is rejected with clear error
- **Category:** error
- **Scenario:** SC-001
- **Requirements:** FR-001, FR-006
- **Preconditions:**
  - Valid credentials configured
- **Steps:**
  - Given GOOGLE_CLIENT_ID="test-client-id", GOOGLE_CLIENT_SECRET="test-secret"
  - When user runs: `mcp-proxy -u http://mcp.test.com` (HTTP, not HTTPS)
  - Then mcp-proxy exits with code 1
  - And stderr contains: "Error: MCP server URL must use HTTPS. Insecure HTTP connections are not allowed."
  - And no connection attempt is made
- **Cleanup:** None
- **Priority:** High

#### E2E-014: Malformed MCP server URL
- **Category:** error
- **Scenario:** All
- **Requirements:** FR-001, FR-006
- **Preconditions:**
  - Valid credentials
- **Steps:**
  - Given GOOGLE_CLIENT_ID="test-client-id", GOOGLE_CLIENT_SECRET="test-secret"
  - When user runs: `mcp-proxy -u not-a-valid-url`
  - Then mcp-proxy exits with code 1
  - And stderr contains: "Error: Invalid MCP server URL format"
- **Cleanup:** None
- **Priority:** High

#### E2E-015: Cannot create token cache directory
- **Category:** error
- **Scenario:** All
- **Requirements:** FR-003, FR-006
- **Preconditions:**
  - ~/.cache/ directory exists but is read-only
- **Steps:**
  - Given ~/.cache/ has permissions 0555 (read-only)
  - When OAuth flow completes and mcp-proxy attempts to save tokens
  - Then mcp-proxy exits with code 4
  - And stderr contains: "Error: Cannot create token cache directory at ~/.cache/mcp-proxy/: permission denied"
- **Cleanup:** Restore ~/.cache/ permissions
- **Priority:** High

#### E2E-016: Cannot write token file (disk full simulation)
- **Category:** error
- **Scenario:** All
- **Requirements:** FR-003, FR-006
- **Preconditions:**
  - ~/.cache/mcp-proxy/ directory exists
  - Disk write will fail (simulated)
- **Steps:**
  - Given ~/.cache/mcp-proxy/ exists
  - When OAuth flow completes and mcp-proxy attempts to write token file
  - And write operation fails (disk full simulation)
  - Then mcp-proxy exits with code 4
  - And stderr contains: "Error: Cannot save tokens to ~/.cache/mcp-proxy/{url_base64}.json: no space left on device"
- **Cleanup:** None
- **Priority:** Medium

### Side Effect Tests

#### E2E-017: Token file created with correct permissions
- **Category:** side effect
- **Scenario:** SC-001
- **Requirements:** FR-003
- **Preconditions:**
  - No cached token
  - OAuth flow will succeed (simulated in this test)
- **Steps:**
  - Given no cached token exists
  - When token storage writes a token file (simulated write operation)
  - Then token file is created at ~/.cache/mcp-proxy/{url_base64}.json
  - And file permissions are exactly 0600 (owner read/write only)
  - And directory ~/.cache/mcp-proxy/ has permissions 0700
  - And file contains valid JSON with required fields
- **Cleanup:** Delete token file
- **Priority:** Critical

#### E2E-023: URL with special characters (base64url encoding)
- **Category:** edge case
- **Scenario:** All
- **Requirements:** FR-001
- **Preconditions:**
  - Valid credentials
- **Steps:**
  - Given MCP server URL: "https://mcp.test.com/path?query=value&other=123"
  - When token filename is generated
  - Then token filename is base64url-encoded URL with .json extension
  - And filename does not contain: /, ?, &, =
  - And filename is valid on all platforms (Linux, macOS, Windows)
  - And token file can be read back successfully
- **Cleanup:** Delete token file
- **Priority:** Medium

#### E2E-028: Token file permissions enforced
- **Category:** security
- **Scenario:** All
- **Requirements:** FR-003
- **Preconditions:**
  - Token file will be created
- **Steps:**
  - Given OAuth flow will complete (simulated)
  - When token file is created
  - Then file permissions are set to 0600 before writing content
  - And directory permissions are 0700
  - And no other users can read the file
  - And permissions are verified after write
- **Cleanup:** Delete token file
- **Priority:** Critical

#### E2E-029: Sensitive data never in error messages
- **Category:** security
- **Scenario:** All
- **Requirements:** FR-006
- **Preconditions:**
  - Various error conditions
- **Steps:**
  - Given various error scenarios (invalid config, file system errors)
  - When errors occur
  - Then stderr never contains: access_token, refresh_token, client_secret
  - And error messages are descriptive but sanitized
- **Cleanup:** None
- **Priority:** High

#### E2E-031: Token file always has valid JSON
- **Category:** data integrity
- **Scenario:** All
- **Requirements:** FR-003
- **Preconditions:**
  - Token operations will occur
- **Steps:**
  - Given any token write operation (create or update)
  - When token file is written
  - Then file always contains valid JSON
  - And JSON can be parsed successfully
  - And all required fields are present: access_token, expiration_time
  - And expiration_time is valid ISO 8601 UTC timestamp
  - And refresh_token is present if provided
- **Cleanup:** Delete token file
- **Priority:** High

## Constraints

### Files Not to Touch
- None (greenfield project)

### Dependencies Not to Add
- Only standard library packages allowed for this story
- No external CLI frameworks unless absolutely necessary (prefer standard `flag` package)
- No external JSON libraries (use `encoding/json`)

### Patterns to Avoid
- Do not cache configuration in global variables
- Do not use relative paths for token storage
- Do not log sensitive data (tokens, secrets)
- Do not use non-atomic file writes

### Scope Boundary
- **NOT in this story:** OAuth flow, HTTP servers, MCP server communication, token refresh logic
- **IN this story:** CLI parsing, token file I/O, error handling framework, base64url encoding

## Non Regression

### Existing Tests That Must Pass
- None (greenfield project)

### Behaviors That Must Not Change
- None (greenfield project)

### API Contracts to Preserve
- None (greenfield project)
