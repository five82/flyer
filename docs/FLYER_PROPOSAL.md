# Flyer TUI Proposal

## Overview
Flyer is a read-only terminal dashboard for monitoring a single Spindle daemon running on a trusted home network. It surfaces queue health, foreground/background progress, and live logs in a k9s-inspired layout without exposing mutating actions. The focus is fast situational awareness for one developer/operator.

## Scope & Assumptions
- Single Spindle instance discovered via the standard config file; no profile selector.
- No authentication layers—the TUI talks to the local API/IPC endpoints assumed to be reachable only on the home network or through Tailscale.
- Read-only feature set at launch: view queue state, inspect item metadata, follow daemon and per-item logs. Retry/reset/clear commands remain in the CLI.
- No export or external monitoring integrations. If needed later, data hooks can be layered on top of the polling client.

## Data Sources
- HTTP API (`/api/status`, `/api/queue`, `/api/queue/{id}`) for queue items, counts, dependencies, and last item metadata.
- Optional IPC socket (`spindle.sock`) for richer telemetry in future iterations, but v1 will rely on HTTP for simplicity.
- Log files under the configured `log_dir` (default `~/.local/share/spindle/logs/`), primarily `spindle.log` plus background item logs referenced via `BackgroundLogPath` when present.
- Config values taken from `~/.config/spindle/config.toml` with sensible defaults when keys are missing.

## User Experience
- Status bar refreshed every ~2 seconds showing daemon state, PID, queue totals (Pending, Processing, Failed, Review, Completed), and timestamp of last update.
- Main layout: left table lists queue items (ID, title, status, lane, progress). Selection updates the right-side detail pane with metadata, file paths, and error context.
- Bottom log dock tails the daemon log, with a toggle to switch to the selected item’s background log when available. A keybinding (`l`) flips between sources.
- Keyboard shortcuts modeled after k9s conventions (`Tab` to move focus, `/` to filter, `?` for help overlay). Filters reset with `Esc`.
- Graceful degradation when the daemon is offline: the UI shows a disconnected banner and surfaces the last successful snapshot (if any).

## Implementation Outline
1. **Config & Client Layer** – Minimal TOML loader for `api_bind`/`log_dir`; HTTP client with polling loop (2 s interval, jittered) and JSON DTOs mirroring Spindle responses.
2. **State Store** – Goroutine-safe snapshot structure delivering queue lists, stats, dependencies, and log buffers to the UI.
3. **UI Components** – tview-based layout (status bar, queue table, detail view, log pane, command/help modals) with incremental updates to avoid redraw flicker.
4. **Log Tailer** – Lightweight reader that trims to the last 400 lines per source and supports follow mode without over-allocating memory.
5. **App Wiring** – Command entrypoint (`cmd/flyer/main.go`) that loads config, constructs clients, starts pollers, and runs the tview application with clean shutdown on `Ctrl+C`.

## Next Steps
- Implement the Go module and baseline UI according to this plan.
- Iterate with hands-on usage against the local Spindle daemon, tightening ergonomics before introducing write actions or multi-instance support.
