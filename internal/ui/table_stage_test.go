package ui

import (
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

func TestFormatQueueRow_PrefersCompletedOverStaleProgressStage(t *testing.T) {
	vm := &viewModel{theme: defaultTheme()}
	item := spindle.QueueItem{
		Status: "completed",
		Progress: spindle.QueueProgress{
			Stage: "organizing",
		},
	}

	got := vm.formatQueueRow(item, false)
	if !strings.Contains(got, "Completed") {
		t.Fatalf("formatQueueRow = %q, want stage to include %q", got, "Completed")
	}
}

func TestFormatQueueRow_PrefersFailedOverProgressStage(t *testing.T) {
	vm := &viewModel{theme: defaultTheme()}
	item := spindle.QueueItem{
		Status: "failed",
		Progress: spindle.QueueProgress{
			Stage: "encoding",
		},
	}

	got := vm.formatQueueRow(item, false)
	if !strings.Contains(got, "Failed") {
		t.Fatalf("formatQueueRow = %q, want stage to include %q", got, "Failed")
	}
}
