# docker-agent-tail

CLI tool that auto-discovers Docker containers, tails their logs, and writes
structured files optimized for AI coding agents.

## Tech Stack

- Go (CLI) — uses Docker SDK (`github.com/docker/docker/client`), `spf13/pflag` for CLI flags
- Next.js 15 + Tailwind v4 (docs site in `docs/`)

## Commands

```sh
# Build
go build -o docker-agent-tail .

# Test
go test ./...
go test -race ./...

# Lint
golangci-lint run

# Test release (no publish)
goreleaser release --snapshot --clean

# Docs site
cd docs && npm run dev
```

## Project Structure

```
├── main.go                    # CLI entrypoint
├── internal/
│   ├── docker/                # Docker client, discovery, events, log streaming
│   ├── session/               # Session dirs, symlink management, file writers
│   └── filter/                # Exclude/mute regex filtering
├── docs/                      # Next.js docs site (separate CLAUDE.md)
└── .goreleaser.yml
```

## Code Conventions

- **`spf13/pflag`** for CLI flags — POSIX-style `--long` and `-s` short flags, no cobra (no subcommands)
- **`context.Context` first param** on all functions that do I/O or Docker SDK calls
- **Error wrapping**: `fmt.Errorf("doing X: %w", err)` — always add context
- **Imports**: stdlib, blank line, external, blank line, internal
- **Interfaces near consumers**, not near implementations
- **Table-driven tests** with `t.Run` subtests; mark safe tests `t.Parallel()`

## Docker SDK Patterns

- Socket discovery order: `$DOCKER_HOST` → `~/.docker/run/docker.sock` → `/var/run/docker.sock`
- Use `stdcopy.StdCopy` for stream demux — Docker multiplexes stdout/stderr
- Always honor `context.Context` cancellation for clean shutdown
- Container log streams are `io.ReadCloser` — always defer Close

## Log Output Format

Log files on disk use JSON Lines (`.jsonl`). Terminal output remains human-readable.

Plain text container output (`<name>.jsonl` / `combined.jsonl`):
```json
{"ts":"2026-03-04T10:30:01.789Z","container":"api","stream":"stdout","message":"GET /api/users 200 12ms"}
```

Structured JSON from containers is merged with metadata:
```json
{"ts":"2026-03-04T10:30:01.789Z","container":"api","stream":"stdout","level":"info","msg":"GET /api/users","status":200}
```

- ISO 8601 timestamps (RFC3339Nano)
- Auto-detection: JSON objects get metadata merged; plain text is wrapped in envelope
- Stream type `stdout`/`stderr` always present

## Testing

- Mock Docker SDK via interfaces (don't hit real daemon in unit tests)
- Integration tests use `testcontainers-go` — tagged `//go:build integration`
- Test `internal/filter` and `internal/session` as pure unit tests
- Always run with `-race` in CI

## Commit Conventions

- **Atomic commits**: One logical change per commit — a single feature, bug fix, or refactor. Don't mix unrelated changes.
- **Tests ship with code**: Every commit that adds or changes behavior must include corresponding tests. Tests and implementation go in the same commit, not separately.
- **All checks must pass before committing**: Run `go build ./...`, `go test -race ./...`, and `golangci-lint run` — all must pass. Do not commit code that breaks the build, fails tests, or has lint violations.
- **Scope by package/concern**: Prefer commits scoped to a single package (e.g., `internal/docker`) or a single cross-cutting concern. If a change touches 3+ packages, consider whether it can be split.
- **Commit message format**: Imperative mood, lowercase, no period. First line under 72 chars. Add a blank line and body for non-trivial changes explaining *why*.

## Important Gotchas

- Docker log driver must be `json-file` or `journald` to support `ContainerLogs` — detect and warn otherwise
- `stdcopy.StdCopy` fails on TTY-attached containers (raw stream) — detect `Config.Tty` and skip demux
- Container names from Docker API include leading `/` — always `strings.TrimPrefix`
