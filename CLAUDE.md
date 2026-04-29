# mcp-proxy — AI Development Guide

## Overview

CLI authenticating proxy between a local stdio MCP client and a remote MCP server (HTTPS, OAuth2.1 + PKCE, JSONL OpenTelemetry tracing).

**Tech Stack:** Go 1.26+, OpenTelemetry SDK, standard library `net/http`, `log/slog`, `flag`.

## Key Commands

```bash
make build                    # Build for current platform → bin/
make run CMD=mcp-proxy ARGS='--url https://...'
make test-unit                # Go unit tests
make test-race                # Race-detector tests
make fmt                      # go fmt
make lint                     # golangci-lint (falls back to go vet)
make check                    # fmt + vet + lint + tests
make clean                    # Remove bin/
```

## Project Structure

```
mcp-proxy/
├── cmd/mcp-proxy/main.go     # Entry point (wiring only)
├── internal/
│   ├── app/                  # Run() orchestration
│   ├── apperr/               # Typed errors, sentinels, exit codes
│   ├── config/               # Flag/env parsing
│   ├── oauth/                # PKCE, discovery, exchange, callback, browser
│   ├── proxy/                # stdio + HTTP, handler, SSE
│   ├── telemetry/            # OpenTelemetry SDK + JSONL stdout exporter
│   └── token/                # Atomic storage, refresh, expiration
├── Dockerfile                # Multi-stage scratch image
├── docker-compose.yml
├── .golangci.yml
└── .agent_docs/              # Loaded on demand
```

## Essential Conventions

- **HTTP clients are injected.** No package-level `http.Client`; pass `*http.Client` so tests use `httptest.Server.Client()` and prod uses TLS-verified defaults.
- **All HTTP requests use `http.NewRequestWithContext`.**
- **Error types** in `internal/apperr` (`AppError` with `Unwrap()`, exit codes 1-5, sentinels `ErrTokenFileNotFound`, `ErrInvalidTokenFormat`, `ErrTokenMissingField`).
- **Logging** uses `log/slog` (`slog.SetDefault` in `app.Run`). Never log tokens, secrets, prompts.
- **Telemetry** uses `internal/telemetry` (OTel wrapper). Spans: `oauth.token_check`, `oauth.token_refresh`, `http.forward`. Trace file: `~/.cache/mcp-proxy/traces.jsonl` (0600).
- **Magic numbers are constants** at the top of each file (port range, timeouts, file perms, PKCE entropy).
- **Token files**: `~/.cache/mcp-proxy/{base64url(server_url)}.json` with 0600 perms; dir 0700; atomic writes (temp + rename).
- **Tests live next to packages** as `*_test.go` in external `package _test` form.

## Documentation Index

- `.agent_docs/golang.md` — Go coding conventions
- `.agent_docs/makefile.md` — Makefile target reference
- `README.md` — User-facing documentation
- `user-stories/` — Implementation backlog (US-001…US-004)
- `specs/` — Specifications

## Status

US-001 + US-002 + US-003 + US-004 implemented. OAuth2.1 with PKCE, token refresh + automatic 401 retry, stdio↔HTTP streaming, OpenTelemetry tracing, graceful shutdown.
