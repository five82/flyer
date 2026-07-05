package ui

import (
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

// sectionOrder asserts that each name appears in got, in the given order.
func sectionOrder(t *testing.T, got string, names ...string) {
	t.Helper()
	last := -1
	for _, name := range names {
		idx := strings.Index(got, name)
		if idx == -1 {
			t.Fatalf("overview missing section %q, got:\n%s", name, got)
		}
		if idx < last {
			t.Fatalf("section %q out of order, got:\n%s", name, got)
		}
		last = idx
	}
}

func overviewFor(t *testing.T, item spindle.QueueItem) string {
	t.Helper()
	m := New(Options{ThemeName: "slate"})
	return stripANSI(m.renderDetailContent(item, 100))
}

func TestOverviewActiveItem_FixedSkeleton(t *testing.T) {
	got := overviewFor(t, spindle.QueueItem{
		ID:    1,
		Stage: "encoding",
		Tasks: []spindle.Task{
			{Type: "ripping", State: "done"},
			{Type: "encoding", State: "running", Progress: spindle.TaskProgress{Percent: 42, Message: "pass 1"}},
		},
		Encoding: &spindle.EncodingStatus{
			Percent:             42,
			Resolution:          "1920x1080",
			Preset:              "6",
			Encoder:             "svt-av1",
			EstimatedTotalBytes: 4 << 30,
		},
		PrimaryAudioDescription: "TrueHD 7.1",
	})

	sectionOrder(t, got, "Pipeline", "Media", "Output")
	if strings.Contains(got, "Attention") {
		t.Fatalf("healthy item must not render Attention, got:\n%s", got)
	}
	for _, want := range []string{"Encoding", "42%", "1920x1080", "TrueHD 7.1", "svt-av1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("overview missing %q, got:\n%s", want, got)
		}
	}
}

func TestOverviewFailedItem_AttentionAfterPipeline(t *testing.T) {
	got := overviewFor(t, spindle.QueueItem{
		ID:           2,
		Stage:        "failed",
		ErrorMessage: "ffmpeg exited 1",
		Tasks: []spindle.Task{
			{Type: "encoding", State: "failed", Error: "ffmpeg exited 1", Attempts: 3},
		},
	})

	sectionOrder(t, got, "Pipeline", "Attention")
	if !strings.Contains(got, "ffmpeg exited 1") {
		t.Fatalf("overview missing error message, got:\n%s", got)
	}
}

func TestOverviewReviewItem_ShowsReasons(t *testing.T) {
	got := overviewFor(t, spindle.QueueItem{
		ID:            3,
		Stage:         "encoding",
		NeedsReview:   true,
		ReviewReasons: []string{"subtitle no-match"},
	})

	sectionOrder(t, got, "Pipeline", "Attention")
	if !strings.Contains(got, "subtitle no-match") {
		t.Fatalf("overview missing review reason, got:\n%s", got)
	}
}

func TestOverviewCompletedItem_OutputResults(t *testing.T) {
	got := overviewFor(t, spindle.QueueItem{
		ID:    4,
		Stage: "completed",
		Tasks: []spindle.Task{
			{Type: "encoding", State: "done"},
		},
		Encoding: &spindle.EncodingStatus{
			OriginalSize:          20 << 30,
			EncodedSize:           5 << 30,
			SizeReductionPercent:  75,
			AverageSpeed:          3.2,
			EncodeDurationSeconds: 3600,
			Validation: &spindle.EncodingValidation{
				Passed: true,
				Steps:  []spindle.EncodingValidationStep{{Name: "duration", Passed: true}},
			},
		},
	})

	sectionOrder(t, got, "Pipeline", "Output")
	for _, want := range []string{"75% reduction", "3.2x avg", "Passed · 1/1 checks"} {
		if !strings.Contains(got, want) {
			t.Fatalf("overview missing %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Attention") {
		t.Fatalf("completed healthy item must not render Attention, got:\n%s", got)
	}
}

func TestOverviewTVItem_EpisodeSummary(t *testing.T) {
	episodes := make([]spindle.EpisodeStatus, 4)
	for i := range episodes {
		episodes[i].Key = string(rune('a' + i))
		episodes[i].Episode = i + 1
	}
	got := overviewFor(t, spindle.QueueItem{
		ID:       5,
		Stage:    "ripping",
		Episodes: episodes,
	})

	sectionOrder(t, got, "Pipeline", "Episodes")
	if !strings.Contains(got, "4 planned") {
		t.Fatalf("overview missing episode summary, got:\n%s", got)
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  []string
	}{
		{"short passes through", "hello world", 20, []string{"hello world"}},
		{"wraps at word boundary", "alpha beta gamma", 11, []string{"alpha beta", "gamma"}},
		{"hard-splits long words", "abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"zero width passes through", "hello", 0, []string{"hello"}},
		{"empty input", "", 10, []string{""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapText(tc.in, tc.width)
			if len(got) != len(tc.want) {
				t.Fatalf("wrapText() = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("wrapText() = %q, want %q", got, tc.want)
				}
			}
		})
	}
}
