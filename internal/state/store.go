package state

import (
	"errors"
	"sync"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

// Snapshot represents the latest data available to the UI.
type Snapshot struct {
	Status      spindle.StatusResponse
	HasStatus   bool
	Queue       []spindle.QueueItem
	LastUpdated time.Time
	LastError   error
}

// Store coordinates concurrent updates to the snapshot.
type Store struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

// Update replaces the stored snapshot. When err is non-nil the previous data is
// kept but the error is recorded for visibility.
func (s *Store) Update(status *spindle.StatusResponse, queue []spindle.QueueItem, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.snapshot.LastError = err
		s.snapshot.LastUpdated = time.Now()
		return
	}

	s.snapshot.Queue = cloneQueue(queue)
	if status != nil {
		s.snapshot.Status = *status
		s.snapshot.HasStatus = true
	} else {
		s.snapshot.HasStatus = false
	}
	s.snapshot.LastError = nil
	s.snapshot.LastUpdated = time.Now()
}

// Snapshot returns a copy of the current snapshot.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := s.snapshot
	copy.Queue = cloneQueue(s.snapshot.Queue)
	if s.snapshot.LastError != nil {
		copy.LastError = errors.New(s.snapshot.LastError.Error())
	}
	return copy
}

func cloneQueue(items []spindle.QueueItem) []spindle.QueueItem {
	if len(items) == 0 {
		return nil
	}
	dup := make([]spindle.QueueItem, len(items))
	copy(dup, items)
	return dup
}
