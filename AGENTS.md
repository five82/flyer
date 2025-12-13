# Repository Guidelines

## Notes For LLM Coding Agents
- Run `./check-ci.sh` before and after changes; keep it aligned with `.github/workflows/ci.yml`.
- Tests must not read the real home directory or Spindle config; use `t.TempDir()` plus `t.Setenv("HOME", ...)` and/or pass explicit config paths.
- Prefer unit tests for pure helpers/formatters; avoid event-loop/terminal integration tests unless explicitly requested.
- Do not `git commit` or `git push` unless explicitly instructed to.

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
1. **Toolchain** – Use Go 1.25.x (run `go version`). `asdf install golang 1.25.3 && asdf local golang 1.25.3` keeps everyone on the same patch.
2. **Sync deps** – After `git pull origin main`, run `go mod tidy` to align `go.sum`.
3. **Fast safety net** – Run `./check-ci.sh` to mirror GitHub Actions locally; for faster iterations you can still use `golangci-lint run ./...` and `go test ./internal/...` between UI touches.
4. **Manual smoke** – `go run ./cmd/flyer` (≈2s) launches the TUI against your current Spindle config. Use `watchexec -- go run ./cmd/flyer` if you want auto-reload while iterating on UI.
5. **Format** – Finish every session with `gofmt -w $(git ls-files '*.go')` (or run on touched files) plus `goimports -w <files>` if you have it installed.

## Project Structure & Module Organization
Flyer is a small Go 1.25 project. Keep the entrypoint in `cmd/flyer/main.go`, with orchestration in `internal/app`. Config and Spindle discovery helpers live in `internal/config`, HTTP polling clients in `internal/spindle`, and UI components in `internal/ui`. Log helpers sit in `internal/logtail`. Tests reside next to their packages (for example, `internal/ui/table_test.go`).

## Development Workflow & Commands
- Baseline local CI: `./check-ci.sh` (mirrors GitHub Actions).
- `golangci-lint run ./...` is the fast safety net; `go test ./internal/...` exercises the business logic without hitting slower integration edges; `go test ./...` is the release gate. Expect these to complete in under 3 seconds on a laptop.
- `./check-ci.sh` runs the same basic checks as CI with a minimal environment to catch missing toolchain issues early.
- `go run ./cmd/flyer` or `go build ./cmd/flyer` should succeed even without a live Spindle daemon; missing config falls back to defaults defined in `internal/config/doc.go`.
- Use `watchexec --restart -- go run ./cmd/flyer` during UI work so the TUI refreshes automatically. If you do not have Watchexec, install via `cargo install watchexec-cli` or your package manager.
- Update dependencies with `go get` and immediately follow with `go mod tidy`; reviewers expect tidy diffs with no stray requirements.

## Coding Style & Naming Conventions
Run `gofmt`/`goimports` on every change. Stick to tabs for indentation, 100-column guideline, and lower-case package names. Exported symbols follow CamelCase; keep helpers unexported unless needed by another package. Add brief doc comments only where behavior is non-obvious; avoid over-documenting trivial functions.

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
Flyer is intentionally read-only: no queue mutations, retries, or clears. Assume a single Spindle daemon on a trusted network, defaulting to `~/.config/spindle/config.toml` for discovery. There is no authentication or multi-profile support planned in the near term—optimize for simplicity and one-operator workflows.
