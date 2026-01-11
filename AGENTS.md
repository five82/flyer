# AGENTS.md

This file provides guidance when working with code in this repository.

CLAUDE.md and GEMINI.md are symlinks to this file so all agent guidance stays in one place.
Do not modify this header.

Do not run `git commit` or `git push` unless explicitly instructed.

# Repository Guidelines

## Notes For LLM Coding Agents
- Run `./check-ci.sh` before and after changes; keep it aligned with `.github/workflows/ci.yml`.
- Tests must not read the real home directory or Spindle config; use `t.TempDir()` plus `t.Setenv("HOME", ...)` and/or pass explicit config paths.
- Prefer unit tests for pure helpers/formatters; avoid event-loop/terminal integration tests unless explicitly requested.

## Related Repos (Local Dev Layout)

Flyer is one of three sibling repos that are developed together on this machine:

- **flyer** (this repo): `~/projects/flyer/` — read-only TUI for Spindle
- **spindle**: `~/projects/spindle/` — daemon + CLI; Flyer polls Spindle’s `api_bind` endpoint and tails logs for display
- **drapto**: `~/projects/drapto/` — encoder invoked by Spindle; Flyer does not call Drapto directly

GitHub:

- flyer - https://github.com/five82/flyer
- spindle - https://github.com/five82/spindle
- drapto - https://github.com/five82/drapto

## Agent Quick Start
1. **Toolchain** – Use Go 1.25.x (see `go.mod` + `.github/workflows/ci.yml`). If you use asdf, pick a Go 1.25 patch and pin it locally (example: `asdf install golang 1.25.5 && asdf local golang 1.25.5`).
2. **Sync deps** – After `git pull origin main`, run `go mod tidy` to align `go.sum`.
3. **Fast safety net** – Run `./check-ci.sh` to mirror GitHub Actions locally; for faster iterations you can still use `golangci-lint run ./...` and `go test ./internal/...` between UI touches. CI pins golangci-lint `v2.5.0` (install via `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.5.0`).
4. **Manual smoke** – `go run ./cmd/flyer` (≈2s) launches the TUI against your current Spindle config. Use `watchexec -- go run ./cmd/flyer` if you want auto-reload while iterating on UI.
5. **Format** – Finish every session with `gofmt -w $(git ls-files '*.go')` (or run on touched files) plus `goimports -w <files>` if you have it installed.

## Project Structure & Module Organization
Flyer is a small Go 1.25 project. Keep the entrypoint in `cmd/flyer/main.go`, with orchestration in `internal/app`. Config and Spindle discovery helpers live in `internal/config`, HTTP polling clients in `internal/spindle`, and UI components in `internal/ui`. Log helpers sit in `internal/logtail`. User preferences (theme selection) live in `internal/prefs`. Tests reside next to their packages (for example, `internal/ui/table_test.go`).

## Architecture Overview

Flyer follows a simple polling architecture with clear separation between data fetching and UI rendering:

```
┌─────────────────────────────────────────────────────────────────┐
│  cmd/flyer/main.go                                              │
│    └─> app.Run()                                                │
│          ├─> config.Load()      Load Spindle config (TOML)      │
│          ├─> prefs.Load()       Load Flyer preferences          │
│          ├─> spindle.NewClient() Create HTTP client             │
│          ├─> state.Store{}      Shared state container          │
│          ├─> StartPoller()      Background polling goroutine    │
│          └─> ui.Run()           Start TUI (blocks)              │
└─────────────────────────────────────────────────────────────────┘

Background Poller (2s interval):       UI Loop (1s refresh):
┌──────────────────────────────┐      ┌──────────────────────────┐
│ spindle.FetchStatus()        │      │ store.Snapshot()         │
│ spindle.FetchQueue()         │      │ logtail.Read()           │
│ store.Update() ─────────────────────│ render TUI               │
└──────────────────────────────┘      └──────────────────────────┘
```

Key design decisions:
- **state.Store**: Thread-safe container using `sync.RWMutex` for producer-consumer pattern
- **Snapshot isolation**: UI receives immutable copies, preventing concurrent modification
- **Graceful degradation**: On poll failures, old data is retained with error displayed

## Development Workflow & Commands
- Baseline local CI: `./check-ci.sh` (mirrors GitHub Actions).
- `golangci-lint run ./...` is the fast safety net; `go test ./internal/...` exercises the business logic without hitting slower integration edges; `go test ./...` is the release gate. Expect these to complete in under 3 seconds on a laptop.
- `./check-ci.sh` runs the same basic checks as CI with a minimal environment to catch missing toolchain issues early.
- `go run ./cmd/flyer` or `go build ./cmd/flyer` should succeed even without a live Spindle daemon; missing config falls back to defaults defined in `internal/config/doc.go`.
- Use `watchexec --restart -- go run ./cmd/flyer` during UI work so the TUI refreshes automatically. If you do not have Watchexec, install via `cargo install watchexec-cli` or your package manager.
- Update dependencies with `go get` and immediately follow with `go mod tidy`; reviewers expect tidy diffs with no stray requirements.

## Coding Style & Naming Conventions
Run `gofmt`/`goimports` on every change. Stick to tabs for indentation, 100-column guideline, and lower-case package names. Exported symbols follow CamelCase; keep helpers unexported unless needed by another package. Add brief doc comments only where behavior is non-obvious; avoid over-documenting trivial functions.

## Theme Guidelines
Flyer supports multiple color themes defined in `internal/ui/theme.go`. When modifying themes:

1. **Use official palettes only**—do not invent colors
2. **Follow established UI hierarchies**—reference canonical implementations for how colors map to UI surfaces
3. **Maintain proper contrast**—backgrounds should be dark enough for text readability

### Nightfox Theme
- **Spec**: https://github.com/EdenEast/nightfox.nvim

| Role | Color | Hex | Usage |
|------|-------|-----|-------|
| bg0 | — | `#131a24` | Outermost background |
| bg1 | — | `#192330` | Main content panels |
| bg2 | — | `#212e3f` | Secondary surfaces |
| bg3 | — | `#29394f` | Focus/active states |
| fg1 | — | `#cdcecf` | Primary text (cool gray) |
| comment | — | `#738091` | Muted text (3.3:1 contrast) |
| fg3 | — | `#71839b` | Dimmest text (3.1:1 contrast) |
| Accents | blue | `#719cd6` | Focus borders, accent |
| | cyan | `#63cdcf` | Info |
| | green | `#81b29a` | Success |
| | yellow | `#dbc074` | Warning |
| | red | `#c94f6d` | Error/danger |

### Kanagawa Theme
- **Spec**: https://github.com/rebelot/kanagawa.nvim

| Role | Color | Hex | Usage |
|------|-------|-----|-------|
| sumiInk0 | — | `#16161D` | Outermost background |
| sumiInk3 | — | `#1F1F28` | Main content panels |
| sumiInk4 | — | `#2A2A37` | Focus/active states, secondary surfaces |
| fujiWhite | — | `#DCD7BA` | Primary text (warm parchment) |
| oldWhite | — | `#C8C093` | Muted text (7.6:1 contrast) |
| fujiGray | — | `#727169` | Dimmest text (2.8:1 contrast) |
| Accents | crystalBlue | `#7E9CD8` | Focus borders, accent |
| | springBlue | `#7FB4CA` | Info |
| | springGreen | `#98BB6C` | Success |
| | carpYellow | `#E6C384` | Warning |
| | waveRed | `#E46876` | Error/danger |

### Slate Theme
- **Palette**: https://tailwindcss.com/docs/colors
- **UI hierarchy**: https://ui.shadcn.com/docs/theming (shadcn/ui canonical implementation)

| Role | Tailwind | Hex | Usage |
|------|----------|-----|-------|
| Background | slate-950 | `#020617` | Outermost background |
| Surface | slate-900 | `#0f172a` | Main content panels |
| SurfaceAlt | slate-800 | `#1e293b` | Secondary surfaces, borders |
| FocusBg | ~slate-750 | `#283548` | Focus/active states (between 800/700) |
| Foreground | slate-100 | `#f1f5f9` | Primary text |
| Muted | slate-400 | `#94a3b8` | Muted text |
| Faint | slate-500 | `#64748b` | Dimmest text |
| Accent | sky-400 | `#38bdf8` | Focus borders, links |

Use semantic color roles (Success, Warning, Danger, Info) mapped to appropriate palette colors for each theme.

## Testing Guidelines
Lean on Go’s standard `testing` package. Prefer table-driven tests for client parsing and UI formatting logic. Mock the HTTP layer with `httptest` when asserting polling behavior; no third-party assertion libs are required. Aim to cover the snapshot store, status formatting, and log trimming utilities. Use `go test ./internal/...` for focused iterations. For code that depends on Spindle configuration, inject explicit config paths or pre-built `config.Config` structs so tests never touch the user’s home directory.

## Environment & Data Notes
- Flyer assumes a single Spindle daemon on a trusted network. Configuration is resolved via `~/.config/spindle/config.toml`, with tilde expansion handled by `internal/config`.
- If you need a local file, drop something like:

```
api_bind = "127.0.0.1:7487"
log_dir = "~/.local/share/spindle/logs"
```

  The package defaults cover missing keys, so you can omit values you do not care about.
- Override the discovery flow with `--config /absolute/path/to/config.toml` when running Flyer, or by passing the path into helper constructors inside tests.
- Sample log files are not required for unit tests; instead, stub the logtail layer or feed it temporary directories via `t.TempDir()`.

## Commit & Pull Request Guidelines
Do not `git commit` or `git push` unless explicitly instructed to. Prefer leaving changes staged/uncommitted and summarize what changed for review.
Write imperative, present-tense subjects under 50 characters with optional wrapped bodies. Reference issues or TODOs inline. Include screenshots or terminal recordings for notable UI updates when preparing PRs. Keep branches rebased on `main`.

## Definition of Done Checklist
- All touched Go files formatted with `gofmt`/`goimports`.
- `./check-ci.sh` passes locally (or equivalently: `go test ./...` + `golangci-lint run ./...`).
- `golangci-lint run ./...` and `go test ./internal/...` and `go test ./...` pass locally.
- `go run ./cmd/flyer` (or `watchexec` loop) verified the UI change if applicable, with screenshots recorded for PRs.
- README/AGENTS/docs updated if behavior, flags, or environment assumptions changed.

## Scope Notes
Flyer is a small personal hobby project maintained by a single developer. Keep it simple and avoid over-engineering—this is not an enterprise application.

Flyer is intentionally read-only: no queue mutations, retries, or clears. Assume a single Spindle daemon on a trusted network, defaulting to `~/.config/spindle/config.toml` for discovery. There is no authentication or multi-profile support planned—optimize for simplicity and one-operator workflows.

When considering new features, ask: "Does this solve a real problem for the single maintainer's daily use?" If not, skip it.
