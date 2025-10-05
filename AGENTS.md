# Repository Guidelines

## Project Structure & Module Organization
Flyer is a small Go 1.25 project. Keep the entrypoint in `cmd/flyer/main.go`, with orchestration in `internal/app`. Config and Spindle discovery helpers live in `internal/config`, HTTP polling clients in `internal/spindle`, and UI components in `internal/ui`. Log helpers sit in `internal/logtail`. Tests reside next to their packages (for example, `internal/ui/table_test.go`).

## Build, Test, and Development Commands
Use `go mod tidy` after pulling new dependencies. `go build ./cmd/flyer` (or `go run ./cmd/flyer`) compiles the TUI. `go test ./...` executes the suite. During iteration, `watchexec -- go run ./cmd/flyer` keeps the dashboard hot-reloaded if you have Watchexec installed.

## Coding Style & Naming Conventions
Run `gofmt`/`goimports` on every change. Stick to tabs for indentation, 100-column guideline, and lower-case package names. Exported symbols follow CamelCase; keep helpers unexported unless needed by another package. Add brief doc comments only where behavior is non-obvious; avoid over-documenting trivial functions.

## Testing Guidelines
Lean on Go’s standard `testing` package. Prefer table-driven tests for client parsing and UI formatting logic. Mock the HTTP layer with `httptest` when asserting polling behavior; no third-party assertion libs are required. Aim to cover the snapshot store, status formatting, and log trimming utilities. Use `go test ./internal/...` for focused iterations.

## Commit & Pull Request Guidelines
Write imperative, present-tense subjects under 50 characters with optional wrapped bodies. Reference issues or TODOs inline. Include screenshots or terminal recordings for notable UI updates when preparing PRs. Keep branches rebased on `main`.

## Scope Notes
Flyer is intentionally read-only: no queue mutations, retries, or clears. Assume a single Spindle daemon on a trusted network, defaulting to `~/.config/spindle/config.toml` for discovery. There is no authentication or multi-profile support planned in the near term—optimize for simplicity and one-operator workflows.
