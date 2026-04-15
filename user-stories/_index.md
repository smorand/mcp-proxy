# User Stories Index

> Source Specification: specs/2026-04-15_22:30:35-mcp-proxy.md
> Generated on: 2026-04-15
> Total Stories: 4

## Implementation Order

| Order | ID | Title | FRs | Scenarios | Tests | Depends On | Complexity | Status |
|-------|-----|-------|-----|-----------|-------|------------|------------|--------|
| 1 | US-001 | Foundation: CLI & Token File Management | FR-001, FR-003, FR-006 | All (validation) | 11 | — | M | ready |
| 2 | US-002 | OAuth2.1 Flow with PKCE | FR-002 | SC-001, SC-004 | 6 | US-001 | L | ready |
| 3 | US-003 | Token Refresh & Expiration | FR-004 | SC-003, SC-004 | 7 | US-001, US-002 | M | ready |
| 4 | US-004 | MCP Server Proxy & Streaming | FR-005 | SC-002, SC-003, SC-004 | 7 | US-001, US-003 | M | ready |

## Dependency Graph

```
US-001 (Foundation: CLI & Token Management)
  ├──▶ US-002 (OAuth2.1 Flow with PKCE)
  │      └──▶ US-003 (Token Refresh & Expiration)
  │             └──▶ US-004 (MCP Server Proxy & Streaming)
  └──▶ US-003 (Token Refresh & Expiration)
         └──▶ US-004 (MCP Server Proxy & Streaming)
```

## Story Summaries

### US-001: Foundation: CLI & Token File Management
**Complexity:** M | **Tests:** 11 | **Depends On:** none

Establishes the foundational infrastructure: CLI argument parsing with environment variable support (--url, --client-id, --client-secret with "env:" prefix), secure token file management at ~/.cache/mcp-proxy/ with 0600/0700 permissions, base64url filename encoding, and robust error handling framework with specific exit codes (1-5).

**Key Deliverables:**
- CLI parsing with env variable resolution
- Token file I/O with atomic writes
- Error handling with actionable messages
- Base64url encoding for filenames

**Test Coverage:**
- 0 happy path, 5 error, 3 side effect, 1 edge case, 2 security, 1 data integrity

---

### US-002: OAuth2.1 Flow with PKCE
**Complexity:** L | **Tests:** 6 | **Depends On:** US-001

Implements complete OAuth2.1 authorization code flow with PKCE: .well-known endpoint discovery, cryptographically secure PKCE code generation (43-128 chars, SHA256), temporary HTTP callback server (localhost:3000-3010), browser integration, and token exchange. Enables first-time authentication and token acquisition.

**Key Deliverables:**
- .well-known discovery
- PKCE code_verifier and code_challenge generation
- Temporary HTTP server for OAuth callback
- Browser integration (open, xdg-open, start)
- Token exchange with PKCE verification

**Test Coverage:**
- 1 happy path, 2 error, 1 side effect, 2 edge case, 1 security

---

### US-003: Token Refresh & Expiration
**Complexity:** M | **Tests:** 7 | **Depends On:** US-001, US-002

Implements automatic token expiration detection and refresh logic using refresh_token. Compares current time with expiration_time, automatically refreshes expired tokens, and gracefully falls back to full OAuth re-authentication when refresh tokens are invalid. Provides seamless user experience with minimal authentication interruptions.

**Key Deliverables:**
- Token expiration detection
- Automatic refresh using refresh_token
- Fallback to full OAuth on refresh failure
- Atomic token file updates

**Test Coverage:**
- 2 happy path, 2 error, 1 side effect, 1 edge case, 2 data integrity

---

### US-004: MCP Server Proxy & Streaming
**Complexity:** M | **Tests:** 7 | **Depends On:** US-001, US-003

Implements the core MCP proxy functionality: stdio interface for local MCP clients, forwards JSON-RPC messages to remote MCP servers via streamable HTTP with OAuth2.1 authentication, handles 401 responses with automatic token refresh, and integrates OpenTelemetry tracing to ~/.cache/mcp-proxy/traces.jsonl. Completes the end-to-end user experience.

**Key Deliverables:**
- stdio interface (read stdin, write stdout)
- Streamable HTTP to MCP server
- Authorization header injection
- 401 handling with automatic refresh
- OpenTelemetry tracing (JSONL format)
- Graceful shutdown (SIGINT handling)

**Test Coverage:**
- 1 happy path, 3 error, 1 side effect, 3 edge case

## Traceability

Every FR from the spec MUST appear in exactly one user story.
Every E2E test from the spec MUST appear in exactly one user story.
Every scenario from the spec MUST be covered by at least one user story.

### Coverage Verification

**Functional Requirements:**
- FRs in spec: 6 (FR-001 through FR-006)
- FRs assigned to stories: 6
- Unassigned: none ✅

**E2E Tests:**
- E2E tests in spec: 31
- Tests assigned to stories: 31
- Unassigned: none ✅

**Scenarios:**
- Scenarios in spec: 4 (SC-001 through SC-004)
- Scenarios covered: 4
- Uncovered: none ✅

### FR to Story Mapping

| FR | Title | Story |
|----|-------|-------|
| FR-001 | CLI Argument Parsing | US-001 |
| FR-002 | OAuth2.1 Discovery and Authentication | US-002 |
| FR-003 | Token File Management | US-001 |
| FR-004 | Token Expiration and Refresh | US-003 |
| FR-005 | MCP Server Proxy | US-004 |
| FR-006 | Error Handling and User Feedback | US-001 |

### E2E Test to Story Mapping

| Test ID | Category | Story |
|---------|----------|-------|
| E2E-001 | Happy Path | US-002 |
| E2E-002 | Happy Path | US-004 |
| E2E-003 | Happy Path | US-003 |
| E2E-004 | Happy Path | US-003 |
| E2E-005 | Error | US-001 |
| E2E-006 | Error | US-001 |
| E2E-007 | Error | US-001 |
| E2E-008 | Error | US-002 |
| E2E-009 | Error | US-004 |
| E2E-010 | Error | US-004 |
| E2E-011 | Error | US-003 |
| E2E-012 | Error | US-003 |
| E2E-013 | Error | US-002 |
| E2E-014 | Error | US-001 |
| E2E-015 | Error | US-001 |
| E2E-016 | Error | US-001 |
| E2E-017 | Side Effect | US-001 |
| E2E-018 | Side Effect | US-003 |
| E2E-019 | Side Effect | US-002 |
| E2E-020 | Side Effect | US-004 |
| E2E-021 | Edge Case | US-002 |
| E2E-022 | Edge Case | US-004 |
| E2E-023 | Edge Case | US-001 |
| E2E-024 | Edge Case | US-003 |
| E2E-025 | Edge Case | US-004 |
| E2E-026 | Edge Case | US-004 |
| E2E-027 | Security | US-002 |
| E2E-028 | Security | US-001 |
| E2E-029 | Security | US-001 |
| E2E-030 | Data Integrity | US-003 |
| E2E-031 | Data Integrity | US-001 |

### Scenario to Story Mapping

| Scenario | Title | Stories |
|----------|-------|---------|
| SC-001 | First-time use (no cached token) | US-001, US-002 |
| SC-002 | Subsequent use with valid token | US-001, US-004 |
| SC-003 | Token expired, refresh available | US-001, US-003, US-004 |
| SC-004 | Token expired, no valid refresh | US-001, US-002, US-003, US-004 |

## Implementation Notes

### Story US-001: Foundation
- **Start here:** This story has no dependencies and provides the base layer for all other stories
- **Critical for:** CLI parsing, token file I/O, error handling framework
- **Estimated effort:** 2-3 days
- **Key risk:** File permission handling across platforms (Linux, macOS, Windows)

### Story US-002: OAuth Flow
- **Requires:** US-001 completed (CLI parsing, token storage, error handling)
- **Critical for:** First-time authentication, token acquisition
- **Estimated effort:** 3-4 days
- **Key risk:** Browser integration across platforms, port availability (3000-3010)

### Story US-003: Token Refresh
- **Requires:** US-001 (token storage), US-002 (OAuth flow for fallback)
- **Critical for:** Seamless user experience, automatic token management
- **Estimated effort:** 2-3 days
- **Key risk:** Atomic file updates, race conditions

### Story US-004: MCP Proxy
- **Requires:** US-001 (config, errors), US-003 (token refresh for 401 handling)
- **Critical for:** End-to-end functionality, user-facing feature
- **Estimated effort:** 3-4 days
- **Key risk:** Streaming performance, memory usage, OpenTelemetry integration

### Total Estimated Effort
- **Minimum:** 10 days (2+3+2+3)
- **Maximum:** 14 days (3+4+3+4)
- **Recommended:** Plan for 12 days with buffer

## Next Steps

1. **Review and approve** this slicing plan
2. **Start with US-001** (Foundation: CLI & Token Management)
3. **Implement stories in order** (US-001 → US-002 → US-003 → US-004)
4. **Run full test suite** after each story to ensure non-regression
5. **Update documentation** (README.md, CLAUDE.md, .agent_docs/) as you go

## Success Criteria

A story is considered **complete** when:
- ✅ All acceptance tests pass (100% pass rate)
- ✅ All non-regression tests pass (from previous stories)
- ✅ Code follows established patterns
- ✅ No new dependencies added (unless explicitly allowed)
- ✅ Error messages are clear and actionable
- ✅ Security requirements met (permissions, no sensitive data in logs)
