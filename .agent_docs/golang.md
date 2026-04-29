# Go Coding Conventions (mcp-proxy)

Project-specific notes; for the canonical Go style refer to `~/.claude/skills/golang/SKILL.md`.

## Layout

- `cmd/mcp-proxy/main.go` — wiring only, calls `app.Run()`.
- `internal/app/` — orchestrates configuration, telemetry, OAuth, proxy run loop.
- `internal/{domain}/` — small domain packages: `apperr`, `config`, `oauth`, `proxy`, `telemetry`, `token`.

## Errors

- Use `internal/apperr` for user-facing errors. Constructors map to exit codes 1–5.
- `AppError.Unwrap()` exposes the cause; use `errors.Is` / `errors.As`.
- Sentinels: `apperr.ErrTokenFileNotFound`, `apperr.ErrInvalidTokenFormat`, `apperr.ErrTokenMissingField`, `token.ErrRefreshRejected`.
- Wrap with `%w` when propagating; `%v` only for end-of-chain user messages.
- Never include access tokens, refresh tokens, client secrets, or PKCE verifiers in error strings.

## HTTP

- Functions accept `ctx context.Context` first and `*http.Client` for transport injection.
- Use `http.NewRequestWithContext`. Never `http.NewRequest` in this codebase.
- TLS verification is on by default. For tests, pass `httptest.Server.Client()` which trusts the server's self-signed cert.
- Timeouts come from constants in `internal/app/app.go` (`httpClientTimeout`, `mcpHTTPTimeout`).

## Telemetry

- `internal/telemetry` wraps `go.opentelemetry.io/otel`. Spans use the `attribute.KeyValue` type; helpers `StringAttr`, `IntAttr`, `Int64Attr`, `BoolAttr` are pass-throughs.
- Span lifecycle: `tracer.StartSpan(ctx, "name", attrs...)` returns `(ctx, *SpanContext)`; call `span.End(err)`. Errors set the OTel `Error` status and record the error.
- Sensitive values must never be set as attributes.

## Logging

- Use `log/slog` (`slog.Info`, `slog.Warn`, `slog.Error`). Initialised in `app.Run`.
- Pass key/value pairs; never use `fmt.Sprintf` in messages.
- User-facing CLI prompts (e.g. "opening browser") are info-level slog messages to stderr.

## Concurrency

- Pass `ctx` everywhere. Use `signal.Notify` only in `app.signalContext` to keep `main` minimal.
- Goroutines must respect `ctx.Done()` and not leak.

## Tests

- External test packages (`package foo_test`).
- Table-driven where there are >2 cases.
- Use `t.TempDir()` and `t.Setenv` instead of manual cleanup.
- TLS test servers: `httptest.NewTLSServer` + `server.Client()`; never set `InsecureSkipVerify`.

## Forbidden

- `crypto/tls` `InsecureSkipVerify: true` in production code paths.
- Calling `http.NewRequest` without context.
- Storing a `context.Context` in a struct.
- Package-level mutable state (the only exception is `flag.CommandLine` driven by `config.Parse`).
- `init()` functions.
