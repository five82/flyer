package ui

import (
	"strings"
	"testing"
)

func TestEpisodeStageChip_UsesExpectedColors(t *testing.T) {
	th := defaultTheme()
	vm := &viewModel{theme: th}

	t.Run("ripping", func(t *testing.T) {
		got := vm.episodeStageChip("ripping", false)
		wantPrefix := "[" + th.Base.Background + ":" + th.StatusColor("ripping") + "]"
		if !strings.HasPrefix(got, wantPrefix) {
			t.Fatalf("episodeStageChip(ripping) = %q, want prefix %q", got, wantPrefix)
		}
		if !strings.Contains(got, "WORK") {
			t.Fatalf("episodeStageChip(ripping) = %q, want WORK label", got)
		}
	})

	t.Run("ripped", func(t *testing.T) {
		got := vm.episodeStageChip("ripped", false)
		wantPrefix := "[" + th.Base.Background + ":" + th.StatusColor("ripped") + "]"
		if !strings.HasPrefix(got, wantPrefix) {
			t.Fatalf("episodeStageChip(ripped) = %q, want prefix %q", got, wantPrefix)
		}
		if !strings.Contains(got, "RIPD") {
			t.Fatalf("episodeStageChip(ripped) = %q, want RIPD label", got)
		}
	})

	t.Run("failed", func(t *testing.T) {
		got := vm.episodeStageChip("ripped", true)
		wantPrefix := "[" + th.Base.Background + ":" + th.Text.Danger + "]"
		if !strings.HasPrefix(got, wantPrefix) {
			t.Fatalf("episodeStageChip(ripped, failed=true) = %q, want prefix %q", got, wantPrefix)
		}
		if !strings.Contains(got, "FAIL") {
			t.Fatalf("episodeStageChip(ripped, failed=true) = %q, want FAIL label", got)
		}
	})
}
