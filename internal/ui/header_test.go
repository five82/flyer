package ui

import (
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
	"github.com/five82/flyer/internal/state"
)

func TestBuildErrorPartsShowsWorkflowLastError(t *testing.T) {
	theme := GetTheme("Nightfox")
	styles := theme.Styles()
	model := Model{
		theme: theme,
		snapshot: state.Snapshot{
			Status: spindle.StatusResponse{
				Workflow: spindle.WorkflowStatus{LastError: "queue persistence failed"},
			},
		},
	}

	parts := model.buildErrorParts(false, styles)
	if len(parts) != 1 {
		t.Fatalf("error parts = %d, want 1", len(parts))
	}
	got := parts[0]
	if !strings.Contains(got, "WORKFLOW") {
		t.Fatalf("workflow label missing from %q", got)
	}
	for _, word := range []string{"queue", "persistence", "failed"} {
		if !strings.Contains(got, word) {
			t.Fatalf("workflow error word %q missing from %q", word, got)
		}
	}
}
