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
//	1. Allocate ring buffer of size maxLines
//	2. For each line in file:
//	   - Store line at current index
//	   - Increment index (wrapping at maxLines)
//	   - Track total lines seen
//	3. If total < maxLines:
//	   - Return first 'count' entries from buffer
//	4. If total >= maxLines:
//	   - Return buffer starting from current index (oldest line)
//
// This ensures the last maxLines are always available without multiple passes.
//
// # Colorization
//
// The colorization functions parse log lines and apply tview color tags based
// on semantic meaning. The color scheme is designed for readability on dark
// terminal backgrounds.
//
// Recognized log patterns:
//
//   - Timestamps: Dim gray (#666666) - low visual weight
//   - Log levels: Color-coded with bold emphasis
//     • INFO: Green (standard convention)
//     • WARN: Yellow (attention needed)
//     • ERROR: Red (critical issues)
//     • DEBUG: Cyan (diagnostic info)
//   - Components: Cornflower blue (#6495ED) - [component] tags
//   - Item references: Orchid (#DA70D6) - "Item #X (name)" patterns
//   - Separators: Light gray (#AAAAAA) - "–" between fields
//   - Detail lines: White - indented continuation lines
//
// Expected log format:
//
//	2024-10-10 14:32:15 INFO [encoder] Item #42 (Movie Name) – encoding started
//	    - detail information
//	    - additional context
//
// # Color Tag Format
//
// Colors use tview's tag syntax:
//
//   - [color]text[-] - Set foreground color
//   - [color:b]text[-] - Bold text with color
//   - [#RRGGBB]text[-] - Hex color code
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
