# docker-agent-tail

Auto-discover Docker containers, tail their logs, and write structured JSONL files that AI coding agents can actually read.

[![Build](https://github.com/Mscotello/docker-agent-tail/actions/workflows/build.yml/badge.svg)](https://github.com/Mscotello/docker-agent-tail/actions/workflows/build.yml)
[![Release](https://github.com/Mscotello/docker-agent-tail/actions/workflows/release.yml/badge.svg)](https://github.com/Mscotello/docker-agent-tail/releases)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

## The problem

You're debugging a multi-container app. Your AI agent needs to see logs from 8 services at once. You're copy-pasting `docker logs` output into chat windows and losing context.

**docker-agent-tail** fixes this. One command tails every container and writes structured JSONL to disk — ready for Claude Code, Cursor, or any tool that reads files.

```bash
docker-agent-tail --all
```

That's it. Logs stream to your terminal and to `logs/latest/combined.jsonl`. Your agent reads the file, sees every container, every timestamp, every error.

## Quick Install

```bash
# Homebrew (macOS/Linux)
brew install Mscotello/tap/docker-agent-tail

# Direct download
curl -sSL https://github.com/Mscotello/docker-agent-tail/releases/latest/download/install.sh | bash

# Or via Go
go install github.com/Mscotello/docker-agent-tail@latest
```

## What it does

```bash
# Tail all running containers
docker-agent-tail --all

# Tail specific containers
docker-agent-tail --names api,web,db

# Auto-discover from docker-compose
docker-agent-tail --compose

# Filter out noise
docker-agent-tail --all --exclude 'healthcheck|ping' --since 5m
```

Terminal output is human-readable. Disk output is structured JSONL:

```json
{"ts":"2026-03-04T10:30:01.789Z","container":"api","stream":"stdout","message":"GET /api/users 200 12ms"}
{"ts":"2026-03-04T10:30:02.100Z","container":"api","stream":"stderr","level":"error","msg":"connection pool exhausted","active":48,"max":50}
{"ts":"2026-03-04T10:30:02.456Z","container":"mongodb","stream":"stdout","level":"info","msg":"Connection accepted","attr":{"remote":"192.168.1.1"}}
```

Every line is valid JSON. Structured container output (like MongoDB's JSON logs) is automatically merged with metadata. Log levels from any format — `"level":"info"`, `"severity":"WARNING"`, MongoDB's `"s":"I"` — are normalized to canonical values (`debug`, `info`, `warning`, `error`, `fatal`, `trace`).

## lnav: the best way to read your logs

[lnav](https://lnav.org) is a terminal-based log viewer that turns your JSONL files into a searchable, filterable, SQL-queryable interface. docker-agent-tail has first-class lnav support.

### Zero-setup integration

The lnav format definition is **auto-installed** on first run. Just install lnav and go:

```bash
brew install lnav          # macOS
# apt install lnav         # Linux

docker-agent-tail lnav     # opens latest session in lnav
```

Or point lnav at any log file directly:

```bash
lnav logs/latest/combined.jsonl     # latest session
lnav logs/latest/*.jsonl            # all per-container files
lnav logs/2026-03-04-143700/        # specific session
```

### What you can do with lnav

**Filter by level** — instantly hide noise:
```
:set-min-log-level warning          # hide debug and info
:filter-in error                    # show only errors
```

**Filter by container** — focus on one service:
```
:filter-in api                      # show only the api container
:filter-out mongodb                 # hide mongodb logs
```

**SQL queries** — aggregate and analyze across all containers:
```sql
;SELECT container, count(*) FROM log GROUP BY container ORDER BY count(*) DESC
;SELECT container, level, count(*) FROM log GROUP BY container, level
;SELECT * FROM log WHERE level = 'error' ORDER BY log_time DESC LIMIT 20
```

**Regex search** — find patterns across all logs:
```
/timeout|connection refused
/status.*5\d{2}
```

**Timeline view** — press `t` to see a histogram of log density over time. Spot bursts of errors at a glance.

### How it compares

| Approach | Structured? | Filterable? | SQL? | Multi-container? |
|----------|------------|-------------|------|-----------------|
| `docker logs` | No | No | No | One at a time |
| `docker compose logs` | No | No | No | Interleaved, no filtering |
| `docker-agent-tail` + grep | Yes (JSONL) | Regex | No | All at once |
| **`docker-agent-tail` + lnav** | **Yes (JSONL)** | **Regex + level** | **Yes** | **All at once** |

### Manual format management

The format auto-installs, but you can also manage it manually:

```bash
docker-agent-tail lnav-install                    # install/reinstall format
docker-agent-tail lnav                            # open latest session
docker-agent-tail lnav --session 2026-03-04-143700  # open specific session
```

## AI agent workflow

```bash
# 1. Start tailing in the background
docker-agent-tail --all &

# 2. Tell your agent to read the logs
#    "Read logs/latest/combined.jsonl and help me debug the errors"

# 3. Agent reads structured JSONL, sees all containers, timestamps, levels
#    No copy-paste. No context loss. No token waste on formatting noise.
```

Set up agent config files for Claude Code, Cursor, or Windsurf:

```bash
docker-agent-tail init
```

## Usage reference

```
docker-agent-tail [FLAGS] [PATTERN...]

PATTERN  glob pattern to match container names (supports *, ?, [abc])

Commands:
  init          Set up AI agent config files (skills, CLAUDE.md)
  agent-help    Print usage guide for AI coding agents
  clean         Remove old log sessions (--retain N, --dry-run)
  lnav          Open latest session in lnav (--session NAME)
  lnav-install  Install lnav format for viewing logs with lnav

Flags:
  -a, --all                   Tail all running containers
  -n, --names strings         Explicit container names (comma-separated)
  -c, --compose               Auto-discover from compose project
  -f, --follow                Reattach on container restart (default: false)
      --json                  Output logs as JSON Lines to stdout
  -e, --exclude strings       Regex patterns to exclude log lines
  -m, --mute strings          Hide matching regex from terminal (still written to log files)
  -o, --output string         Output directory (default: "./logs")
  -s, --since string          Start from duration ago (e.g., "5m", "1h")
      --no-color              Disable terminal colors
  -v, --version               Show version and exit
  -h, --help                  Show this help message

Positional Arguments:
  PATTERN                     Glob pattern to filter containers by name (e.g., "web-*")
```

## Log output format

Files on disk: JSON Lines (`.jsonl`). Terminal: human-readable with colors.

Each session creates a timestamped directory under `./logs/`:

| File | Description |
|------|-------------|
| `combined.jsonl` | All containers interleaved |
| `<container>.jsonl` | Per-container logs |
| `metadata.json` | Session info (start time, command, containers) |

A `latest` symlink always points to the most recent session.

### Plain text container output

```json
{"ts":"2026-03-04T10:30:01.789Z","container":"api","stream":"stdout","message":"GET /api/users 200 12ms"}
```

### Structured JSON container output (auto-merged)

```json
{"ts":"2026-03-04T10:30:01.789Z","container":"api","stream":"stdout","level":"info","msg":"request completed","status":200}
```

Level normalization maps common formats to canonical values:

| Source | Normalized |
|--------|-----------|
| `I`, `info`, `notice` | `info` |
| `W`, `warn`, `warning` | `warning` |
| `E`, `err`, `error` | `error` |
| `F`, `fatal`, `critical`, `crit`, `emerg` | `fatal` |
| `D`, `debug` | `debug` |
| `T`, `trace` | `trace` |

## Session management

```bash
docker-agent-tail clean              # keep 5 most recent sessions (default)
docker-agent-tail clean --retain 10  # keep 10 most recent
docker-agent-tail clean --retain 0   # delete all sessions
docker-agent-tail clean --dry-run    # list what would be deleted
```

## Pattern matching

Container name patterns use glob syntax:

| Pattern | Matches |
|---------|---------|
| `api-*` | `api-server`, `api-gateway`, `api-worker` |
| `*-db` | `postgres-db`, `mysql-db` |
| `web-[12]` | `web-1`, `web-2` |
| `???` | Any 3-character container name |

```bash
docker-agent-tail 'api-*'                  # all api containers
docker-agent-tail 'web-*' 'api-*'          # multiple patterns
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | No containers found matching criteria |
| 3 | Docker daemon error (connection failed, timeout) |
| 64 | Usage error (invalid flags or arguments) |

## Docker requirements

- Docker daemon running and accessible via socket
- Docker log driver must be `json-file` or `journald` — other drivers (e.g., `syslog`, `awslogs`) don't support `ContainerLogs` streaming
- Minimum Docker API: auto-negotiated (no minimum version)
- Socket discovery order: `$DOCKER_HOST` → `~/.docker/run/docker.sock` → `/var/run/docker.sock`

## Troubleshooting

**No logs appearing?** Check the container's log driver: `docker inspect --format='{{.HostConfig.LogConfig.Type}}' <container>`. Must be `json-file` or `journald`.

**Truncated log lines?** Lines longer than 1MB are truncated by the scanner buffer. This covers virtually all log payloads.

**Permission denied?** Add your user to the docker group (`sudo usermod -aG docker $USER`) or check Docker socket permissions.

**Docker socket not found?** Set `DOCKER_HOST` to your Docker socket path, or ensure Docker Desktop is running.

**Timed out connecting?** The tool waits 30 seconds for the Docker daemon. Check if Docker is running: `docker info`.

## Known limitations

- TTY-attached containers send raw streams (no stdout/stderr demux)
- Maximum line size: 1MB (larger lines are truncated)
- Only file-based log drivers (`json-file`, `journald`) are supported
- Container names from Docker API include leading `/` — automatically trimmed

## Documentation

Full documentation at [docker-agent-tail.michaelscotello.com](https://docker-agent-tail.michaelscotello.com)

## Development

```bash
make build              # build binary
make test               # run tests with race detector
make lint               # run golangci-lint
make release-snapshot   # test goreleaser (no publish)
```

## License

MIT License — see [LICENSE](LICENSE) for details.
