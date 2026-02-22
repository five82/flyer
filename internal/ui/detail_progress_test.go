package ui

import (
	"testing"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

func TestEstimateETA_SuppressedDuringAnalyzing(t *testing.T) {
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	m.stageFirstSeen[1] = stageObservation{
		stage:     "analyzing",
		firstSeen: time.Now().Add(-10 * time.Minute),
	}
	item := spindle.QueueItem{
		ID:       1,
		Progress: spindle.QueueProgress{Stage: "Analyzing", Percent: 50},
	}
	if got := m.estimateETA(item); got != "" {
		t.Fatalf("expected empty ETA during Analyzing, got %q", got)
	}
}

func TestEstimateETA_SuppressedBelowThreshold(t *testing.T) {
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	m.stageFirstSeen[1] = stageObservation{
		stage:     "ripping",
		firstSeen: time.Now().Add(-5 * time.Minute),
	}
	item := spindle.QueueItem{
		ID:       1,
		Progress: spindle.QueueProgress{Stage: "Ripping", Percent: 4},
	}
	if got := m.estimateETA(item); got != "" {
		t.Fatalf("expected empty ETA at 4%%, got %q", got)
	}
}

func TestEstimateETA_UsesStageEntryTime(t *testing.T) {
	// Stage started 10 minutes ago, item was created 2 hours ago.
	// With percent at 50%, ETA should be ~10m (based on stage time),
	// NOT ~2h (based on creation time).
	stageStart := time.Now().Add(-10 * time.Minute)
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	m.stageFirstSeen[1] = stageObservation{
		stage:     "ripping",
		firstSeen: stageStart,
	}
	item := spindle.QueueItem{
		ID:        1,
		CreatedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		Progress:  spindle.QueueProgress{Stage: "Ripping", Percent: 50},
	}
	got := m.estimateETA(item)
	if got == "" {
		t.Fatal("expected non-empty ETA at 50%")
	}
	// The ETA should be roughly 10m (stage elapsed 10m at 50% → 10m remaining).
	// Definitely should NOT be ~2h. Check it contains "10m" or similar.
	// Since formatDuration rounds, just ensure it doesn't say hours.
	if got == "" {
		t.Fatal("expected ETA string")
	}
	// Sanity: ETA should be short, not contain "h" (which would indicate
	// it incorrectly used creation time 2h ago).
	for _, ch := range got {
		if ch == 'h' {
			t.Fatalf("ETA %q looks too large; stage-based ETA should be ~10m, not hours", got)
		}
	}
}

func TestEstimateRemainingFromProgress_SuppressedDuringAnalyzing(t *testing.T) {
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	item := &spindle.QueueItem{
		ID:       1,
		Progress: spindle.QueueProgress{Stage: "Analyzing", Percent: 50},
	}
	if got := m.estimateRemainingFromProgress(item); got != 0 {
		t.Fatalf("expected 0 during Analyzing, got %v", got)
	}
}

func TestEstimateRemainingFromProgress_SuppressedBelowThreshold(t *testing.T) {
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	m.stageFirstSeen[1] = stageObservation{
		stage:     "ripping",
		firstSeen: time.Now().Add(-5 * time.Minute),
	}
	item := &spindle.QueueItem{
		ID:       1,
		Progress: spindle.QueueProgress{Stage: "Ripping", Percent: 3},
	}
	if got := m.estimateRemainingFromProgress(item); got != 0 {
		t.Fatalf("expected 0 at 3%%, got %v", got)
	}
}
