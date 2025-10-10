// Package spindle provides an HTTP client for the Spindle daemon API.
//
// # Overview
//
// This package defines the API client for communicating with the Spindle media
// processing daemon. It handles HTTP communication, JSON serialization, and
// type-safe representation of queue items and daemon status.
//
// # Architecture
//
// The package is split into two files:
//
//   - client.go: HTTP client implementation and request/response handling
//   - types.go: Data structures mirroring the Spindle API schema
//
// # Client Usage
//
// Create a client using the API bind address from configuration:
//
//	client, err := spindle.NewClient("127.0.0.1:7487")
//	if err != nil {
//		log.Fatalf("failed to create client: %v", err)
//	}
//
//	// Fetch daemon status
//	status, err := client.FetchStatus(ctx)
//	if err != nil {
//		log.Printf("status fetch failed: %v", err)
//	}
//
//	// Fetch queue items
//	queue, err := client.FetchQueue(ctx)
//	if err != nil {
//		log.Printf("queue fetch failed: %v", err)
//	}
//
// # API Endpoints
//
// The client currently supports two read-only endpoints:
//
//   - GET /api/status: Daemon status, workflow state, queue statistics
//   - GET /api/queue: List of all queue items with full details
//
// Both endpoints return JSON responses that are decoded into strongly-typed structs.
//
// # Request Handling
//
// All requests:
//   - Use context for cancellation and timeout control
//   - Set Accept: application/json header
//   - Include User-Agent: flyer/0.1 header
//   - Have a 5-second timeout (configurable via http.Client)
//   - Return wrapped errors with context about what failed
//
// # Error Handling
//
// The client distinguishes between several error types:
//
//   - Client initialization errors: Invalid API bind address format
//   - Network errors: Connection refused, timeout, DNS failure
//   - HTTP errors: 4xx/5xx status codes from the API
//   - Deserialization errors: Malformed JSON responses
//
// All errors are wrapped with descriptive context using fmt.Errorf.
//
// Example error messages:
//   - "execute request: dial tcp: connection refused"
//   - "api /api/status returned status 500"
//   - "decode response: unexpected end of JSON input"
//
// # Type System
//
// The package defines several data structures that mirror the Spindle API:
//
// StatusResponse:
//   - Daemon runtime information (PID, running state)
//   - Workflow statistics (queue counts by status)
//   - Dependency health checks (external tools availability)
//   - Stage health (workflow component readiness)
//
// QueueItem:
//   - Core fields: ID, title, source path, status
//   - Progress tracking: Stage, percentage, message
//   - File paths: Ripped, encoded, and final output files
//   - Metadata: Timestamps, error messages, review flags
//
// # Timestamp Parsing
//
// QueueItem provides helper methods for timestamp parsing:
//
//   - ParsedCreatedAt(): Returns time.Time for creation timestamp
//   - ParsedUpdatedAt(): Returns time.Time for last update timestamp
//
// These methods handle multiple timestamp formats:
//   - RFC3339Nano (with nanoseconds)
//   - RFC3339 (ISO 8601)
//   - Spindle format: "2006-01-02 15:04:05" (local timezone)
//
// Invalid or missing timestamps return time.Time{} (zero value).
//
// # URL Construction
//
// The client accepts several API bind formats:
//
//   - "127.0.0.1:7487" → http://127.0.0.1:7487
//   - "http://localhost:8080" → http://localhost:8080
//   - "192.168.1.10:7487" → http://192.168.1.10:7487
//
// The scheme defaults to "http://" if not specified. HTTPS is supported
// but not used in typical single-machine deployments.
//
// # Thread Safety
//
// The Client struct is safe for concurrent use. The underlying http.Client
// handles connection pooling and concurrent requests internally.
//
// # Network Assumptions
//
// This client assumes:
//   - Spindle daemon is on localhost or trusted local network
//   - No authentication required (single-operator deployment)
//   - HTTP is sufficient (no TLS for local communication)
//   - Default timeout of 5 seconds is appropriate
//
// These assumptions align with Flyer's design as a read-only monitoring tool
// for a single-user local daemon.
//
// # Testing Considerations
//
// When testing code that uses this package:
//   - Use httptest.Server to mock the Spindle API
//   - Test both success and error paths
//   - Verify context cancellation behavior
//   - Check handling of malformed JSON responses
//   - Test URL parsing edge cases
//
// # Design Rationale
//
// The package is intentionally minimal:
//   - No caching (poller handles refresh cadence)
//   - No retries (app layer decides retry policy)
//   - No mutations (Flyer is read-only)
//   - No streaming (snapshot-based polling is sufficient)
//
// This keeps the client simple and predictable while meeting all current needs.
package spindle
