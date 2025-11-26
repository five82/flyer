# Flyer

Flyer is a read-only terminal dashboard for the Spindle disc-ripping daemon. It mirrors k9s-style navigation to surface queue activity, progress, and logs for a single Spindle instance running on your home network. No write actions are exposed—the CLI remains the place for retries, clears, or other mutations.

## Features
- **Queue view** showing all items grouped by status with live counts in the header.
- **Detail view** displaying metadata, file paths, per-episode progress, error messages, and review flags for the selected item.
- **Episode tracker** that summarizes each TV episode's stage (planned → ripped → encoded → final) so box sets are easy to follow even before organization completes.
- **Log viewer** supporting daemon and per-item background logs with syntax highlighting.
- **Log search** with vim-style `/` search, `n`/`N` navigation, and regex support.
- **Fast navigation** via single-key commands and Tab cycling between Queue → Detail → Daemon Log → Item Log.
- **Help overlay** (`h`) showing all available keybindings.

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

**Navigation:**
- `q` – switch to Queue view
- `d` – switch to Detail view for the selected item
- `i` – switch to Item Log view for the selected item
- `l` – cycle log sources (Daemon ↔ Item) and switch to log view
- `Tab` – cycle through views: Queue → Detail → Daemon Log → Item Log
- `ESC` – return to Queue view

**Search (in Log view):**
- `/` – start a new search (supports regex)
- `n` – jump to next search match
- `N` – jump to previous search match

**General:**
- `h` – show help overlay with all keybindings
- `e` / `Ctrl+C` – exit Flyer

## Development
- Format with `gofmt`/`goimports` before committing.
- `go build ./cmd/flyer` to compile the binary.
- `go test ./...` to run the test suite.
- For live reloads during UI development, `watchexec -- go run ./cmd/flyer` works well.

### Project Structure
- `cmd/flyer/` – main entrypoint
- `internal/app/` – application orchestration and polling logic
- `internal/config/` – Spindle config discovery and parsing
- `internal/spindle/` – HTTP client for Spindle API and type definitions
- `internal/state/` – thread-safe snapshot store
- `internal/ui/` – TUI components built on tview/tcell
- `internal/logtail/` – log reading and syntax highlighting

Scope Reminder: Flyer assumes a single trusted Spindle deployment and skips auth, exports, and queue mutations by design.
