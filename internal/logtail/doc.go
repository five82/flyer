// Package logtail provides utilities for reading and colorizing log files.
//
// # Overview
//
// This package implements efficient tail-like reading of log files and applies
// syntax highlighting for display in the TUI. It's optimized for reading the
// last N lines from potentially large log files without loading the entire
// file into memory.
//
// # Core Functionality
//
// The package provides two main capabilities:
//
//  1. Read: Efficiently extract the last N lines from a log file
//  2. ColorizeLine/ColorizeLines: Apply tview color codes for syntax highlighting
//
// # Reading Log Files
//
// The Read function uses a ring buffer algorithm to extract the last maxLines
// from a file, regardless of file size. This approach:
//
//   - Scans the file sequentially (one pass)
//   - Uses O(maxLines) memory, not O(file size)
//   - Returns lines in correct chronological order
//   - Handles files larger than available memory
//
// Example usage:
//
//	lines, err := logtail.Read("/var/log/spindle/daemon.log", 400)
//	if err != nil {
//		log.Printf("failed to read log: %v", err)
//	}
//
// # Ring Buffer Algorithm
//
// The read implementation uses a circular buffer of size maxLines:
//
//  1. Allocate ring buffer of size maxLines
//  2. For each line in file:
//     - Store line at current index
//     - Increment index (wrapping at maxLines)
//     - Track total lines seen
//  3. If total < maxLines:
//     - Return first 'count' entries from buffer
//  4. If total >= maxLines:
//     - Return buffer starting from current index (oldest line)
//
// This ensures the last maxLines are always available without multiple passes.
//
// # Colorization
//
// The colorization functions parse log lines and apply tview color tags based
// on semantic meaning. The color scheme follows best practices for terminal
// log highlighting:
//
//   - WCAG AA compliant (4.5:1 contrast on black backgrounds)
//   - Visual hierarchy: important elements stand out, metadata recedes
//   - Color-blind friendly: doesn't rely on red/green alone
//   - Optimized for scanning: levels and messages are primary anchors
//
// Recognized log patterns:
//
//   - Timestamps: Medium gray (#808080) - de-emphasized metadata
//   - Log levels: WCAG-compliant colors with bold emphasis
//   - INFO: Bright green (#5FD75F) - ~6.5:1 contrast
//   - WARN: Gold (#FFD700) - ~9.8:1 contrast
//   - ERROR: Coral red (#FF6B6B) - ~5.1:1 contrast
//   - DEBUG: Sky blue (#87CEEB) - ~7.4:1 contrast
//   - Components: Light blue (#87AFFF) - ~7.2:1 contrast
//   - Item references: Light purple (#D7AFFF) - ~7.8:1 contrast
//   - Separators: Dim gray (#666666) - subtle, decorative only
//   - Detail lines: White - maximum contrast for readability
//
// Expected log format:
//
//	2024-10-10 14:32:15 INFO [encoder] Item #42 (Movie Name) – encoding started
//	    - detail information
//	    - additional context
//
// # Color Tag Format
//
// Colors use tview's tag syntax with explicit black backgrounds:
//
//   - [color:black]text[-:-] - Foreground color with black background
//   - [color:black:b]text[-:-] - Bold text with color and black background
//   - [#RRGGBB:black]text[-:-] - Hex color with black background
//
// The explicit background ensures consistent rendering across the TUI,
// preventing default terminal colors from showing through
//
// # Performance Considerations
//
// Read function:
//   - Buffer size: 64KB initial, 1MB max (configurable in scanner)
//   - Memory usage: O(maxLines × average line length)
//   - Typical usage: 400 lines × ~200 chars = ~80KB memory
//   - Time complexity: O(n) where n = total lines in file
//
// Colorization:
//   - Uses compiled regexes (matched once per line)
//   - StringBuilder for efficient string concatenation
//   - No heap allocations for small color tags
//
// # Error Handling
//
// Read returns nil, nil for non-existent files (graceful degradation).
// Other errors (permission denied, I/O errors) are returned wrapped.
//
// Colorization functions never return errors - malformed lines are
// returned unchanged rather than failing.
//
// # Testing
//
// The package includes comprehensive tests (logtail_test.go) covering:
//   - Ring buffer correctness for various file sizes
//   - Edge cases (empty files, files smaller than maxLines)
//   - Log parsing accuracy
//   - Color tag generation
//
// # Design Rationale
//
// This package is intentionally simple and focused:
//   - No streaming or file watching (that's the poller's job)
//   - No log rotation handling (reads current file only)
//   - No filtering or searching (that's the UI's job)
//   - Pure functions with no global state
//
// The separation of concerns allows each layer to focus on its core
// responsibility: logtail reads and formats, UI handles interaction,
// poller handles updates.
package logtail
