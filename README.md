# Flyer

Flyer is a read-only terminal dashboard for the Spindle disc-ripping daemon. It mirrors k9s-style navigation to surface queue activity, progress, and logs for a single Spindle instance running on your home network. No write actions are exposed—the CLI remains the place for retries, clears, or other mutations.

## Features
- Queue overview grouped by pipeline stage with live counts in the status bar.
- Selectable table of queue items with detail pane showing metadata, file paths, progress, and review flags.
- Toggling log viewer that tails the main `spindle.log` or the selected item’s background log when available.
- Lightweight search filter (`/`) and help overlay (`?`) to keep keyboard workflows fast.

## Requirements
- Go 1.25 or newer.
- A running Spindle daemon with its HTTP API exposed (default `127.0.0.1:7487`).
- Access to the same config and log directories used by Spindle (typically `~/.config/spindle/config.toml` and `~/.local/share/spindle/logs`).

## Installation
Install directly from source using Go:

```bash
go install github.com/five82/flyer/cmd/flyer@latest
```

The binary will land in `$(go env GOBIN)` (or `$(go env GOPATH)/bin` when `GOBIN` is unset); ensure that directory is on your `PATH`.

To build locally without installing system-wide:

```bash
git clone https://github.com/five82/flyer.git
cd flyer
go build ./cmd/flyer
```

## Quick Start
```bash
flyer             # if installed with go install
go run ./cmd/flyer  # when running from a cloned checkout
```

Optional flags:
- `--config /path/to/config.toml` – override the default Spindle config discovery.
- `--poll 3` – change the refresh interval in seconds (defaults to 2 seconds when omitted).

### Key Bindings
- `q` / `Ctrl+C` – quit.
- `Tab` – move focus between queue table and log pane.
- `l` – toggle between daemon log and selected item log.
- `/` – filter queue items (matches title, status, fingerprint, or progress text).
- `?` – help overlay.

## Development
- Format with `gofmt`/`goimports`.
- `go build ./...` to compile.
- `go test ./...` for the test suite once tests are added.
- For live reloads, `watchexec -- go run ./cmd/flyer` pairs well with the UI.

Scope Reminder: Flyer assumes a single trusted Spindle deployment and skips auth, exports, and queue mutations by design.
