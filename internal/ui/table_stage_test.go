package ui

import (
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

func TestFormatStageWithDetail_PrefersCompletedOverStaleProgressStage(t *testing.T) {
	vm := &viewModel{theme: defaultTheme()}
	item := spindle.QueueItem{
		Status: "completed",
		Progress: spindle.QueueProgress{
			Stage: "organizing",
		},
	}

	got := vm.formatStageWithDetail(item, false)
	if !strings.Contains(got, "Completed") {
		t.Fatalf("formatStageWithDetail = %q, want stage to include %q", got, "Completed")
	}
}

func TestFormatStageWithDetail_PrefersFailedOverProgressStage(t *testing.T) {
	vm := &viewModel{theme: defaultTheme()}
	item := spindle.QueueItem{
		Status: "failed",
		Progress: spindle.QueueProgress{
			Stage: "encoding",
		},
	}

	got := vm.formatStageWithDetail(item, false)
	if !strings.Contains(got, "Failed") {
		t.Fatalf("formatStageWithDetail = %q, want stage to include %q", got, "Failed")
	}
}
