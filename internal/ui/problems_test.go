package ui

import (
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

func TestProblemReasonPrefersErrorThenReview(t *testing.T) {
	item := spindle.QueueItem{
		Status:       "failed",
		ErrorMessage: "rip failed",
		ReviewReason: "needs QA",
		Progress: spindle.QueueProgress{
			Message: "something else",
		},
	}
	if got := problemReason(item); got != "rip failed" {
		t.Fatalf("expected error message, got %q", got)
	}

	item.ErrorMessage = ""
	item.NeedsReview = true
	if got := problemReason(item); got != "needs QA" {
		t.Fatalf("expected review reason, got %q", got)
	}

	item.ReviewReason = ""
	item.Progress.Message = "progress"
	if got := problemReason(item); got != "progress" {
		t.Fatalf("expected progress message, got %q", got)
	}
}

func TestAggregateProblemReasons(t *testing.T) {
	entries := []problemEntry{
		{Reason: "timeout", Kind: problemFailed},
		{Reason: "timeout", Kind: problemFailed},
		{Reason: "bad meta", Kind: problemReview},
	}

	got := aggregateProblemReasons(entries)
	expect := "Timeout ×2  |  Bad Meta ×1"
	if got != expect {
		t.Fatalf("expected %q, got %q", expect, got)
	}
}

func TestCollectProblemEntriesOrdersNewestFailedFirst(t *testing.T) {
	queue := []spindle.QueueItem{
		{ID: 1, Status: "failed", ErrorMessage: "older", UpdatedAt: "2024-01-01T10:00:00Z"},
		{ID: 2, Status: "failed", ErrorMessage: "newer", UpdatedAt: "2024-01-02T10:00:00Z"},
		{ID: 3, Status: "review", NeedsReview: true, ReviewReason: "check audio", UpdatedAt: "2024-01-03T10:00:00Z"},
	}

	entries := collectProblemEntries(queue)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Item.ID != 2 || entries[1].Item.ID != 1 || entries[2].Item.ID != 3 {
		t.Fatalf("unexpected order: got IDs %d, %d, %d", entries[0].Item.ID, entries[1].Item.ID, entries[2].Item.ID)
	}
}
