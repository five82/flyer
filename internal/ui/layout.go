package ui

import "time"

// Terminal width thresholds for responsive layouts.
const (
	// LayoutCompactWidth is the threshold below which compact mode is used.
	LayoutCompactWidth = 100

	// LayoutProgressWidth is the minimum width to show progress columns.
	LayoutProgressWidth = 120

	// LayoutUpdatedWidth is the minimum width to show updated timestamps.
	LayoutUpdatedWidth = 150

	// LayoutExtraWideWidth is the threshold for extra-wide layouts.
	LayoutExtraWideWidth = 160
)

// Log display limits.
const (
	// LogFetchLimit is the maximum number of log events to fetch per request.
	LogFetchLimit = 2000

	// LogBufferLimit is the maximum number of log lines to keep in memory.
	LogBufferLimit = 5000
)

// Timing constants.
const (
	// LogRefreshDebounce is the minimum time between log refreshes.
	LogRefreshDebounce = 400 * time.Millisecond

	// LogFetchTimeout is the timeout for log fetch requests.
	LogFetchTimeout = 2 * time.Second

	// DefaultUIInterval is the default UI refresh interval.
	DefaultUIInterval = time.Second
)
