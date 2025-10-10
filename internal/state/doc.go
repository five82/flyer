// Package state provides thread-safe state management for the Flyer application.
//
// # Overview
//
// This package implements a simple but thread-safe store for sharing daemon status
// and queue data between the background poller and the UI. It acts as the
// coordination point where polling updates meet UI rendering.
//
// # Architecture
//
// The package follows a producer-consumer pattern:
//
//	Producer (Poller):             Consumer (UI):
//	┌────────────────┐            ┌────────────────┐
//	│ FetchStatus()  │            │                │
//	│ FetchQueue()   │            │                │
//	│      ↓         │            │                │
//	│ store.Update() │───────────→│ store.Snapshot()│
//	│      ↓         │  (mutex)   │      ↓         │
//	│  repeat...     │            │  render UI     │
//	└────────────────┘            └────────────────┘
//
// The Store mediates between these two independent goroutines, ensuring:
//   - Atomic updates (no partial/torn reads)
//   - No data races (mutex-protected access)
//   - Immutable snapshots (defensive copying)
//
// # Core Types
//
// Store:
//   - Thread-safe container for latest daemon state
//   - Uses sync.RWMutex for concurrent access
//   - Single writer (poller), multiple readers (UI refresh loop)
//
// Snapshot:
//   - Immutable view of state at a point in time
//   - Contains status, queue, timestamps, and error info
//   - Returned by value with defensive copies
//
// # Concurrency Model
//
// The Store uses a readers-writer lock:
//
//   - Update(): Acquires write lock (exclusive access)
//   - Snapshot(): Acquires read lock (concurrent reads allowed)
//
// This allows the UI to read frequently without blocking the poller,
// and vice versa. The lock is held only during copy operations,
// not during network I/O or rendering.
//
// # Update Semantics
//
// The Update method has special error handling behavior:
//
//	// Success case: Replace entire snapshot
//	store.Update(status, queue, nil)
//	→ snapshot.Status = status
//	→ snapshot.Queue = queue
//	→ snapshot.LastError = nil
//	→ snapshot.LastUpdated = now
//
//	// Error case: Keep old data, record error
//	store.Update(nil, nil, err)
//	→ snapshot.Status = <unchanged>
//	→ snapshot.Queue = <unchanged>
//	→ snapshot.LastError = err
//	→ snapshot.LastUpdated = now
//
// This ensures the UI always has the most recent successful data to display,
// while also being informed of polling failures.
//
// # Defensive Copying
//
// Both Update and Snapshot perform deep copies to prevent shared state:
//
//   - Queue slices are cloned (not just slice header)
//   - Error values are copied (not shared pointers)
//   - Status struct is copied by value
//
// This prevents:
//   - Concurrent modification by UI and poller
//   - Accidental mutation of displayed data
//   - Race conditions on slice/map internals
//
// The cost of copying is minimal (hundreds of queue items × small structs)
// and much simpler than alternative coordination strategies.
//
// # Usage Example
//
//	// Poller goroutine:
//	store := &state.Store{}
//	for {
//		status, err1 := client.FetchStatus(ctx)
//		queue, err2 := client.FetchQueue(ctx)
//		err := errors.Join(err1, err2)
//		store.Update(status, queue, err)
//		time.Sleep(interval)
//	}
//
//	// UI goroutine:
//	ticker := time.NewTicker(time.Second)
//	for range ticker.C {
//		snap := store.Snapshot()
//		renderUI(snap)
//	}
//
// # Memory Characteristics
//
// Typical memory usage for Store:
//   - ~100 queue items × 500 bytes/item = ~50KB
//   - StatusResponse struct = ~2KB
//   - Total snapshot = ~52KB
//
// With 2 copies (internal + snapshot), peak memory is ~104KB.
// This is negligible for a desktop application.
//
// # Error Propagation
//
// The Store preserves errors from the poller for UI display:
//
//   - LastError: Most recent polling error (nil on success)
//   - LastUpdated: Timestamp of last Update call
//   - HasStatus: Whether Status field contains valid data
//
// This allows the UI to show helpful messages like:
//   - "Spindle unavailable (last attempt 14:32:15)"
//   - "Waiting for spindle status..."
//   - Normal display (when LastError is nil)
//
// # Testing Considerations
//
// The Store is safe to construct with zero value:
//
//	store := &state.Store{}  // Ready to use immediately
//
// For tests:
//   - No initialization required
//   - Thread-safe from first use
//   - Snapshot() returns zero Snapshot if never updated
//   - Updates are atomic and immediately visible
//
// # Design Rationale
//
// This package intentionally avoids:
//   - Channels (mutex is simpler for single writer/multiple readers)
//   - Complex synchronization (RWMutex is sufficient)
//   - Incremental updates (full snapshot replacement is easier)
//   - Versioning/history (only latest state matters)
//   - Pub/sub (UI polls snapshots on its own schedule)
//
// The design prioritizes:
//   - Simplicity and correctness over maximum performance
//   - Defensive copying over shared mutable state
//   - Explicit concurrency control over implicit coordination
//
// This is appropriate for Flyer's scale (single daemon, low update frequency).
package state
