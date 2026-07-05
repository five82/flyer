package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes styling escape codes so assertions can match on plain text.
func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func TestRenderStructuredProblems_LeadsWithFailedTask(t *testing.T) {
	m := &Model{theme: GetTheme("slate")}
	styles := m.theme.Styles()

	item := &spindle.QueueItem{
		NeedsReview:   true,
		ReviewReasons: []string{"subtitle no-match"},
		Tasks: []spindle.Task{
			{Type: "encoding", State: "failed", Attempts: 3, Error: "ffmpeg exited 1"},
		},
	}

	var b strings.Builder
	m.renderStructuredProblems(&b, item, styles)
	got := stripANSI(b.String())

	failedIdx := strings.Index(got, "Failed Task")
	reviewIdx := strings.Index(got, "Review Reasons")
	if failedIdx == -1 {
		t.Fatalf("renderStructuredProblems() missing Failed Task header, got %q", got)
	}
	if reviewIdx == -1 {
		t.Fatalf("renderStructuredProblems() missing Review Reasons header, got %q", got)
	}
	if failedIdx > reviewIdx {
		t.Fatalf("Failed Task section should precede Review Reasons: got %q", got)
	}
	if !strings.Contains(got, "Encoding") {
		t.Fatalf("renderStructuredProblems() missing task label, got %q", got)
	}
	if !strings.Contains(got, "attempt 3") {
		t.Fatalf("renderStructuredProblems() missing attempts, got %q", got)
	}
	if !strings.Contains(got, "ffmpeg exited 1") {
		t.Fatalf("renderStructuredProblems() missing task error, got %q", got)
	}
}

func TestRenderStructuredProblems_FailedAtStageFallbackWhenTasksAbsent(t *testing.T) {
	m := &Model{theme: GetTheme("slate")}
	styles := m.theme.Styles()

	item := &spindle.QueueItem{
		FailedAtStage: "ripping",
	}

	var b strings.Builder
	m.renderStructuredProblems(&b, item, styles)
	got := stripANSI(b.String())

	if !strings.Contains(got, "Failed Task") {
		t.Fatalf("renderStructuredProblems() missing fallback Failed Task header, got %q", got)
	}
	if !strings.Contains(got, "Ripping") {
		t.Fatalf("renderStructuredProblems() missing fallback stage label, got %q", got)
	}
}

func TestRenderStructuredProblems_NoFailedTaskComposesAsBefore(t *testing.T) {
	m := &Model{theme: GetTheme("slate")}
	styles := m.theme.Styles()

	item := &spindle.QueueItem{
		NeedsReview:   true,
		ReviewReasons: []string{"subtitle no-match"},
		Tasks: []spindle.Task{
			{Type: "encoding", State: "done"},
		},
	}

	var b strings.Builder
	m.renderStructuredProblems(&b, item, styles)
	got := stripANSI(b.String())

	if strings.Contains(got, "Failed Task") {
		t.Fatalf("renderStructuredProblems() should not show Failed Task section, got %q", got)
	}
	if !strings.HasPrefix(strings.TrimLeft(got, " "), "Review Reasons") {
		t.Fatalf("renderStructuredProblems() should lead with Review Reasons as before, got %q", got)
	}
}
