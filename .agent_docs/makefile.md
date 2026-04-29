# Makefile Reference (mcp-proxy)

The Makefile is the skill-standard auto-detecting Makefile. See `~/.claude/skills/golang/references/Makefile` for the upstream copy and `~/.claude/skills/golang/SKILL.md#makefile` for the full target list.

## Variables That Matter Here

| Variable | Default for this project | Notes |
|----------|--------------------------|-------|
| `COMMANDS` | `mcp-proxy` (auto-detected from `cmd/`) | Add a sibling `cmd/<name>` dir to publish a new binary. |
| `MODULE_NAME` | `github.com/smorand/mcp-proxy` (from `go.mod`) | Override only if module path changes. |
| `BUILD_DIR` | `bin` | Per-platform binaries: `bin/mcp-proxy-darwin-arm64`, etc. |
| `PROJECT_NAME` | `mcp-proxy` (basename of CWD) | Used by Docker targets. |

## Common Targets

| Target | What it does |
|--------|--------------|
| `make build` | Incremental build for current platform. |
| `make build-all` | Build all platforms + launcher script. |
| `make run CMD=mcp-proxy ARGS='--url https://...'` | Build + run. |
| `make test-unit` | `go test -v ./...`. |
| `make test-race` | `go test -race -v ./...`. |
| `make fmt` / `make vet` / `make lint` | Code quality. |
| `make check` | `fmt + vet + lint + test-all`. |
| `make install` | Install to `~/.local/bin`. |
| `make docker-build` / `make docker-push` | Multi-stage scratch image (`Dockerfile`). |
| `make clean` / `make clean-all` | Remove `bin/` (and go.mod/go.sum for `clean-all`). |

## Notes specific to mcp-proxy

- `make test` falls back to `make test-unit` because there are no shell-based functional tests yet (`tests/run_tests.sh` is absent).
- `make docker-build` builds the binary into a `scratch` image; the CLI is stdio-based, so `make run-up` is mostly a build-verification convenience.
- macOS binaries are codesigned automatically by the Makefile (`codesign -f -s -`).
