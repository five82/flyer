package ui

import (
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

func TestNormalizeEpisodeStage_AllowsEmptyForFallback(t *testing.T) {
	if got := normalizeEpisodeStage(""); got != "" {
		t.Fatalf("normalizeEpisodeStage(\"\") = %q, want empty string", got)
	}
	if got := normalizeEpisodeStage("   "); got != "" {
		t.Fatalf("normalizeEpisodeStage(\"   \") = %q, want empty string", got)
	}
}

func TestRenderPipelineStatus_MovieCompleted_UsesChecks(t *testing.T) {
	vm := &viewModel{theme: defaultTheme()}
	item := spindle.QueueItem{
		Status:    "completed",
		Progress:  spindle.QueueProgress{Stage: ""},
		FinalFile: "/library/Movie (2025).mkv",
	}

	var b strings.Builder
	vm.renderPipelineStatus(&b, item, spindle.EpisodeTotals{})
	got := b.String()

	if !strings.Contains(got, "✓") {
		t.Fatalf("renderPipelineStatus(completed movie) = %q, want at least one checkmark", got)
	}
	if strings.Contains(got, "◉") {
		t.Fatalf("renderPipelineStatus(completed movie) = %q, want no current-stage indicator", got)
	}
	if !strings.Contains(got, "Planned") || !strings.Contains(got, "Final") {
		t.Fatalf("renderPipelineStatus(completed movie) = %q, want Planned and Final labels", got)
	}
}

func TestNormalizeEpisodeStage_MapsEpisodeIdentificationLabels(t *testing.T) {
	tests := []string{
		"episode_identifying",
		"Episode identification",
		"Episode identification (42%)",
		"episode_identified",
		"Episode identified",
	}
	for _, in := range tests {
		if got := normalizeEpisodeStage(in); got != "encoding" {
			t.Fatalf("normalizeEpisodeStage(%q) = %q, want %q", in, got, "encoding")
		}
	}
}
