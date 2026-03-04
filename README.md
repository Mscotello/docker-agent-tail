# docker-agent-tail

CLI tool that auto-discovers Docker containers, tails their logs, and writes structured files optimized for AI coding agents.

[![Build](https://github.com/scotello/docker-agent-tail/actions/workflows/build.yml/badge.svg)](https://github.com/scotello/docker-agent-tail/actions/workflows/build.yml)
[![Release](https://github.com/scotello/docker-agent-tail/actions/workflows/release.yml/badge.svg)](https://github.com/scotello/docker-agent-tail/releases)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

## Features

- **Auto-discover containers** — Tail all containers, specific by name, or from docker-compose projects
- **Smart log formatting** — Structured output with timestamps and container names, optimized for AI agents
- **Flexible filtering** — Exclude containers and mute streams from terminal output while keeping log files
- **Follow restarts** — Automatically reattach to containers when they restart
- **Cross-platform** — Runs on macOS (Intel/ARM) and Linux (x86_64/ARM64)

## Quick Install

### Homebrew

```bash
brew install scotello/docker-agent-tail/docker-agent-tail
```

### From GitHub Releases

```bash
# Direct download and install
curl -sL https://docker-agent-tail.dev/install.sh | sh
```

### Go Install

```bash
go install github.com/scotello/docker-agent-tail@latest
```

## Quick Start

Tail all running containers:
```bash
docker-agent-tail --all
```

Tail specific containers:
```bash
docker-agent-tail --names api,web,db
```

Tail containers from a docker-compose project:
```bash
docker-agent-tail --compose
```

Tail with filtering:
```bash
docker-agent-tail --all \
  --exclude "healthcheck|debug" \
  --mute "verbose-service" \
  --output ./logs
```

## Usage

```
docker-agent-tail [flags]

Flags:
  -a, --all                   Tail all running containers
  -n, --names strings         Explicit container names (comma-separated)
  -c, --compose               Auto-discover from compose project
  -f, --follow                Reattach on restart (default: true)
  -e, --exclude strings       Regex patterns to exclude containers
  -m, --mute strings          Hide from terminal, still write to file
  -o, --output string         Output directory (default: "./logs")
  -s, --since string          Start from N ago (e.g., "5m", "1h")
  -j, --json                  JSON lines output
      --no-color              Disable terminal colors
  -h, --help                  Show this help message
```

## Log Output Format

Per-container logs (`<container_name>.log`):
```
[2026-03-04T10:30:01.789Z] [stdout] GET /api/users 200 12ms
[2026-03-04T10:30:02.456Z] [stderr] Connection error: timeout
```

Combined logs (`combined.log`):
```
[2026-03-04T10:30:01.789Z] [api    ] [stdout] GET /api/users 200 12ms
[2026-03-04T10:30:02.456Z] [web    ] [stderr] Connection error: timeout
```

- ISO 8601 timestamps with millisecond precision
- Fixed-width container name column for easy alignment
- Stream type (`[stdout]`/`[stderr]`) always included

## Documentation

Full documentation available at [docker-agent-tail.dev](https://docker-agent-tail.dev)

## Development

```bash
# Build
make build

# Test
make test

# Lint
make lint

# Test release (no publish)
make release-snapshot
```

## Requirements

- Docker daemon running and accessible
- Docker log driver must be `json-file` or `journald`
- Go 1.22+ (for building from source)

## License

MIT License — see [LICENSE](LICENSE) for details.
