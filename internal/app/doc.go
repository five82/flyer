// Package app provides the orchestration layer for the Flyer application.
//
// # Overview
//
// This package wires together configuration, polling, state management, and the UI
// to create the complete Flyer TUI experience. It serves as the composition root
// where all dependencies are initialized and connected.
//
// # Architecture
//
// The app package follows a simple initialization pattern:
//
//  1. Load Spindle daemon configuration from ~/.config/spindle/config.toml
//  2. Initialize HTTP client for Spindle API communication
//  3. Create shared state.Store for UI and poller coordination
//  4. Verify Spindle daemon is reachable before starting UI
//  5. Launch background poller goroutine for continuous updates
//  6. Start the TUI and block until user exits or context cancels
//
// # Components
//
//   - app.go: Main Run function and initial daemon availability check
//   - poller.go: Background goroutine that fetches status and queue data periodically
//
// # Data Flow
//
//	┌──────────────┐
//	│   Run()      │ Initialize everything
//	└──────┬───────┘
//	       │
//	       ├─────> config.Load()       Read Spindle config
//	       ├─────> spindle.NewClient() Create HTTP client
//	       ├─────> state.Store{}       Shared state container
//	       ├─────> ensureSpindleAvailable() Pre-flight check
//	       ├─────> StartPoller()       Launch background updates
//	       └─────> ui.Run()            Start TUI (blocks)
//
//	Background Poller Loop:
//	┌─────────────────────────────────────────┐
//	│ StartPoller() goroutine                 │
//	│  ├─> FetchStatus()                      │
//	│  ├─> FetchQueue()                       │
//	│  └─> store.Update()  (atomic)           │
//	│      └─> UI reads store.Snapshot()      │
//	└─────────────────────────────────────────┘
//
// # Polling Behavior
//
// The poller runs continuously in the background at a configurable interval
// (default: 2 seconds). On each tick:
//
//   - Fetches daemon status from Spindle API
//   - Fetches queue items from Spindle API
//   - Updates the shared state.Store atomically
//   - Logs errors but continues polling on failure
//
// The UI reads snapshots from the store at its own refresh rate (typically 1 second).
// This separation allows the UI to remain responsive even during slow API calls.
//
// # Error Handling
//
// The app package distinguishes between fatal and recoverable errors:
//
// Fatal errors (returned from Run):
//   - Configuration file not found or invalid
//   - Spindle client initialization failure
//   - Initial daemon availability check failure (3 second timeout)
//
// Recoverable errors (logged, polling continues):
//   - Periodic status fetch failures
//   - Periodic queue fetch failures
//   - Network timeouts during polling
//
// This ensures Flyer can survive temporary daemon restarts or network hiccups
// while preventing startup against a non-existent daemon.
//
// # Configuration
//
// The Options struct allows callers to customize:
//
//   - ConfigPath: Path to spindle config.toml (default: ~/.config/spindle/config.toml)
//   - PollEvery: Polling interval in seconds (default: 2 seconds)
//
// # Usage Example
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	opts := app.Options{
//		ConfigPath: "", // Use default
//		PollEvery:  2,  // 2 second polling
//	}
//
//	if err := app.Run(ctx, opts); err != nil {
//		log.Fatalf("flyer failed: %v", err)
//	}
//
// # Dependencies
//
//   - config: Loads and parses Spindle configuration files
//   - spindle: HTTP client for Spindle daemon API
//   - state: Thread-safe state container for status and queue data
//   - ui: Terminal user interface (TUI) implementation
//
// # Design Rationale
//
// This package intentionally keeps orchestration logic minimal and focused.
// Business logic lives in domain packages (spindle, config, state, ui).
// The app package simply connects these pieces with sensible defaults for
// the single-operator, read-only monitoring use case.
package app
