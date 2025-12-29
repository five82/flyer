// Package ui provides a terminal user interface for the Flyer application.
//
// # Architecture Overview
//
// The UI package implements a TUI (Terminal User Interface) using the tview library,
// styled after k9s for a familiar Kubernetes-like dashboard experience. The interface
// is read-only and focuses on monitoring Spindle queue items and logs.
//
// # Package Structure
//
// The package is organized into focused modules:
//
//   - ui.go: Core application setup, event handling, layout composition, and the main Run function
//   - logo.go: ASCII art logo generation using figlet with fallback
//   - search.go: Log search functionality with regex pattern matching and highlighting
//   - table.go: Queue table rendering and data formatting utilities
//   - navigation.go: View navigation, detail rendering, and log file management
//
// # Main Components
//
// The viewModel struct serves as the central state container for all UI components:
//
//   - Header section: Status information, command menu, and logo
//   - Main content: Switchable pages for queue table, item details, logs, and problems
//   - Search interface: Vim-style search with pattern highlighting
//
// # View Types
//
// Four main views are available:
//
//   - Queue View: Table of all queue items with ID, title, status, lane, and progress
//   - Detail View: Full details for the selected queue item
//   - Logs View: Real-time log display (daemon logs or per-item background logs)
//   - Problems View: Warning/error log stream for the selected item
//
// # Key Features
//
//   - Real-time updates: Auto-refreshes from state.Store at configurable intervals
//   - Log sources: Toggle between daemon logs and item-specific background logs
//   - Search: Vim-style "/" to search logs with regex, "n/N" to navigate matches
//   - Navigation: Tab cycles through views, arrow keys navigate tables
//   - Color coding: k9s-inspired color scheme for status indicators
//
// # Event Flow
//
//  1. Run() initializes the tview application and viewModel
//  2. Background goroutine polls state.Store and calls viewModel.update()
//  3. User input triggers navigation or search actions
//  4. Views are rendered on-demand when switched
//  5. Context cancellation cleanly shuts down the UI
//
// # External Dependencies
//
//   - state.Store: Provides queue snapshots and daemon status
//   - logtail: Reads and colorizes log files
//   - spindle: Queue item data structures
//   - config: Spindle daemon configuration discovery
//
// # Usage Example
//
//	opts := ui.Options{
//		Store:         stateStore,
//		DaemonLogPath: "/var/log/spindle/daemon.log",
//		Config:        cfg,
//		RefreshEvery:  time.Second,
//	}
//	if err := ui.Run(ctx, opts); err != nil {
//		log.Fatal(err)
//	}
//
// # Key Bindings
//
//   - q: Queue view
//   - d: Focus detail pane for selected item
//   - l: Toggle log source (daemon/item)
//   - i: Show logs for selected item
//   - p: Show problems for selected item
//   - Tab: Cycle through views
//   - /: Start search (in log view)
//   - n/N: Next/previous search match
//   - Space: Toggle log auto-tail (pause/follow)
//   - F: Filter daemon logs (component/lane/request)
//   - End or G: Jump to bottom + follow (log view)
//   - ESC: Return to queue view
//   - e or Ctrl+C: Exit
//
// # Design Principles
//
//   - Read-only interface: No mutations to queue or daemon state
//   - Single operator: No multi-user or authentication support
//   - Simplicity first: Focused feature set, minimal configuration
//   - k9s-inspired: Familiar navigation and visual design for terminal power users
package ui
