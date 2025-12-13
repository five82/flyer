package ui

import (
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

func TestFormatStage_PrefersCompletedOverStaleProgressStage(t *testing.T) {
	vm := &viewModel{theme: defaultTheme()}
	item := spindle.QueueItem{
		Status: "completed",
		Progress: spindle.QueueProgress{
			Stage: "organizing",
		},
	}

	got := vm.formatStage(item)
	if !strings.Contains(got, "Completed") {
		t.Fatalf("formatStage = %q, want stage to include %q", got, "Completed")
	}
}

func TestFormatStage_PrefersFailedOverProgressStage(t *testing.T) {
	vm := &viewModel{theme: defaultTheme()}
	item := spindle.QueueItem{
		Status: "failed",
		Progress: spindle.QueueProgress{
			Stage: "encoding",
		},
	}

	got := vm.formatStage(item)
	if !strings.Contains(got, "Failed") {
		t.Fatalf("formatStage = %q, want stage to include %q", got, "Failed")
	}
}
