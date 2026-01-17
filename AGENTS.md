# AGENTS.md

This file provides guidance when working with code in this repository.

CLAUDE.md and GEMINI.md are symlinks to this file so all agent guidance stays in one place.
Do not modify this header.

## TL;DR

- Do not run `git commit` or `git push` unless explicitly instructed.
- Run `./check-ci.sh` before handing work back.
- Tests must not read real home directory or Spindle config - use `t.TempDir()` + `t.Setenv("HOME", ...)`.
- Use Context7 MCP for library/API docs without being asked.

## Project Snapshot

Flyer is a **read-only TUI** for monitoring Spindle. It polls Spindle's API and tails logs to display queue status, item details, and logs. No queue mutations - use Spindle CLI for that.

- **Scope**: Single-developer hobby project - avoid over-engineering
- **Environment**: Go 1.25+, golangci-lint v2.5.0
- **Operation**: Works without live Spindle daemon (falls back to defaults)

## Related Repos

| Repo | Path | Role |
|------|------|------|
| flyer | `~/projects/flyer/` | Read-only TUI for Spindle (this repo) |
| spindle | `~/projects/spindle/` | Daemon + CLI; Flyer polls its `api_bind` endpoint |
| drapto | `~/projects/drapto/` | Encoder invoked by Spindle; Flyer does not call directly |

GitHub: [flyer](https://github.com/five82/flyer) | [spindle](https://github.com/five82/spindle) | [drapto](https://github.com/five82/drapto)

## Build, Test, Lint

```bash
go run ./cmd/flyer                    # Run TUI
go test ./...                         # Test
golangci-lint run ./...               # Lint
./check-ci.sh                         # Full CI (recommended before handoff)
watchexec --restart -- go run ./cmd/flyer  # Auto-reload during UI work
```

## Architecture

```
cmd/flyer/main.go -> app.Run()
  ├─> config.Load()       # Spindle config (TOML)
  ├─> prefs.Load()        # Flyer preferences
  ├─> spindle.NewClient() # HTTP client
  ├─> state.Store{}       # Thread-safe state (sync.RWMutex)
  ├─> StartPoller()       # Background: FetchStatus/FetchQueue every 2s
  └─> ui.Run()            # TUI blocks, refreshes every 1s from store.Snapshot()
```

**Key patterns:**
- `state.Store`: Thread-safe container; UI receives immutable snapshots
- Graceful degradation: On poll failures, old data retained with error displayed

## Package Map

| Package | Role |
|---------|------|
| `cmd/flyer` | Entry point |
| `internal/app` | Orchestration |
| `internal/config` | Spindle config discovery, tilde expansion |
| `internal/spindle` | HTTP polling client |
| `internal/state` | Thread-safe store |
| `internal/ui` | Bubble Tea TUI components |
| `internal/logtail` | Log file tailing |
| `internal/prefs` | User preferences (theme selection) |

## Testing

- Use `t.TempDir()` + `t.Setenv("HOME", ...)` - never touch real home directory
- Mock HTTP with `httptest` for polling tests
- Table-driven tests for client parsing and UI formatting
- No third-party assertion libs required

## Theme Development

Themes are defined in `internal/ui/theme.go`. Rules:
1. Use official palettes only - do not invent colors
2. Follow established UI hierarchies
3. Maintain proper contrast for readability

See `docs/themes.md` for color tables (Nightfox, Kanagawa, Slate).

## Configuration

Flyer reads Spindle's config from `~/.config/spindle/config.toml`. Relevant keys:
- `api_bind` - Spindle API endpoint (default `127.0.0.1:7487`)
- `log_dir` - Where to tail logs from

Override with `--config /path/to/config.toml` or inject paths in tests.

## Scope Constraints

Flyer is intentionally limited:
- **Read-only**: No queue mutations, retries, or clears
- **Single operator**: No auth, no multi-profile
- **Trusted network**: Assumes single Spindle daemon on localhost

When considering features, ask: "Does this solve a real problem for daily use?" If not, skip it.
