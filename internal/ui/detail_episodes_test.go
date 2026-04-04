package ui

import (
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

func TestShouldAutoExpandEpisodes(t *testing.T) {
	tests := []struct {
		name     string
		item     spindle.QueueItem
		episodes []spindle.EpisodeStatus
		totals   spindle.EpisodeTotals
		want     bool
	}{
		{
			name:     "small sets expand",
			episodes: make([]spindle.EpisodeStatus, 3),
			totals:   spindle.EpisodeTotals{Planned: 3},
			want:     true,
		},
		{
			name:     "failed episodes expand",
			episodes: []spindle.EpisodeStatus{{Key: "a"}, {Key: "b", Status: "failed"}},
			totals:   spindle.EpisodeTotals{Planned: 2},
			want:     true,
		},
		{
			name:     "incomplete matching expands",
			item:     spindle.QueueItem{EpisodeIdentifiedCount: 5},
			episodes: make([]spindle.EpisodeStatus, 10),
			totals:   spindle.EpisodeTotals{Planned: 10},
			want:     true,
		},
		{
			name:     "final mismatch expands",
			item:     spindle.QueueItem{Stage: "completed"},
			episodes: make([]spindle.EpisodeStatus, 10),
			totals:   spindle.EpisodeTotals{Planned: 10, Final: 9},
			want:     true,
		},
		{
			name:     "large healthy set stays collapsed",
			episodes: make([]spindle.EpisodeStatus, 10),
			totals:   spindle.EpisodeTotals{Planned: 10, Final: 10},
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldAutoExpandEpisodes(tc.item, tc.episodes, tc.totals); got != tc.want {
				t.Fatalf("shouldAutoExpandEpisodes() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMatchedEpisodeCount(t *testing.T) {
	episodes := []spindle.EpisodeStatus{{Episode: 1}, {MatchedEpisode: 2}, {Key: "x"}}
	if got := matchedEpisodeCount(spindle.QueueItem{}, episodes); got != 2 {
		t.Fatalf("matchedEpisodeCount() = %d, want 2", got)
	}
	if got := matchedEpisodeCount(spindle.QueueItem{EpisodeIdentifiedCount: 5}, episodes); got != 3 {
		t.Fatalf("matchedEpisodeCount() with explicit count = %d, want 3", got)
	}
}

func TestToggleEpisodesCollapsed_UsesEffectiveDefaultState(t *testing.T) {
	m := New(Options{ThemeName: "slate"})
	m.snapshot.Queue = []spindle.QueueItem{{
		ID:       1,
		Episodes: make([]spindle.EpisodeStatus, 10),
		EpisodeTotals: &spindle.EpisodeTotals{
			Planned: 10,
			Final:   10,
		},
	}}

	item := m.getSelectedItem()
	if item == nil {
		t.Fatal("getSelectedItem() = nil")
	}
	episodes, totals := item.EpisodeSnapshot()
	if !m.isEpisodesCollapsed(*item, episodes, totals) {
		t.Fatal("isEpisodesCollapsed() before toggle = false, want true")
	}

	m.toggleEpisodesCollapsed()

	item = m.getSelectedItem()
	episodes, totals = item.EpisodeSnapshot()
	if m.isEpisodesCollapsed(*item, episodes, totals) {
		t.Fatal("isEpisodesCollapsed() after one toggle = true, want false")
	}
}

func TestTogglePathExpanded_TogglesOnFirstPress(t *testing.T) {
	m := New(Options{ThemeName: "slate"})
	m.snapshot.Queue = []spindle.QueueItem{{ID: 1}}

	if got := m.detailState.pathExpanded[1]; got {
		t.Fatal("pathExpanded before toggle = true, want false")
	}

	m.togglePathExpanded()

	if got := m.detailState.pathExpanded[1]; !got {
		t.Fatal("pathExpanded after one toggle = false, want true")
	}
}

func TestActiveEpisodeDescriptor(t *testing.T) {
	m := Model{theme: GetTheme("slate")}
	episodes := []spindle.EpisodeStatus{{Key: "a", Active: true}, {Key: "b"}}
	idx, inferred, reason := m.activeEpisodeDescriptor(spindle.QueueItem{}, episodes)
	if idx != 0 || inferred || reason != "active" {
		t.Fatalf("activeEpisodeDescriptor() = (%d, %v, %q), want (0, false, %q)", idx, inferred, reason, "active")
	}

	episodes = []spindle.EpisodeStatus{{Key: "a", Stage: "ripped"}, {Key: "b", Stage: "encoded"}}
	idx, inferred, reason = m.activeEpisodeDescriptor(spindle.QueueItem{Stage: "encoding"}, episodes)
	if idx != 0 || !inferred || reason == "" {
		t.Fatalf("activeEpisodeDescriptor() inferred = (%d, %v, %q), want inferred match", idx, inferred, reason)
	}
}

func TestDescribeEpisodeFileStates_IncludesSubtitles(t *testing.T) {
	m := Model{theme: GetTheme("slate")}
	ep := &spindle.EpisodeStatus{
		RippedPath:    "/r.mkv",
		EncodedPath:   "/e.mkv",
		SubtitledPath: "/s.mkv",
		FinalPath:     "/f.mkv",
	}
	if got := m.describeEpisodeFileStates(ep); got != "RIP ENC SUB FIN" {
		t.Fatalf("describeEpisodeFileStates() = %q, want %q", got, "RIP ENC SUB FIN")
	}
}

func TestDescribeEpisodeHelpers(t *testing.T) {
	ep := spindle.EpisodeStatus{MatchScore: 0.93}
	if got := describeEpisodeMapping(ep); got != "Match 0.93" {
		t.Fatalf("describeEpisodeMapping() = %q, want %q", got, "Match 0.93")
	}
	ep = spindle.EpisodeStatus{GeneratedSubtitleDecision: "no_match"}
	if got := describeEpisodeIssue(ep); got != "subtitle no-match" {
		t.Fatalf("describeEpisodeIssue() = %q, want %q", got, "subtitle no-match")
	}
	ep = spindle.EpisodeStatus{}
	if got := describeEpisodeIssue(ep); got != "unconfirmed mapping" {
		t.Fatalf("describeEpisodeIssue() unmatched = %q, want %q", got, "unconfirmed mapping")
	}
}

func TestEpisodeStageChip_UsesSpecificLabels(t *testing.T) {
	m := Model{theme: GetTheme("slate")}
	styles := m.theme.Styles()
	bg := NewBgStyle(m.theme.Background)
	cases := []struct {
		stage string
		label string
	}{
		{stage: "encoding", label: "ENC"},
		{stage: "ripping", label: "RIP"},
		{stage: "episode_identifying", label: "MATCH"},
		{stage: "identifying", label: "ID"},
	}
	for _, tc := range cases {
		if got := m.episodeStageChip(tc.stage, false, styles, bg); !strings.Contains(got, tc.label) {
			t.Fatalf("episodeStageChip(%q) = %q, want label %q", tc.stage, got, tc.label)
		}
	}
}

func TestDescribeItemFileStates_DerivesTotals(t *testing.T) {
	m := Model{theme: GetTheme("slate")}
	item := spindle.QueueItem{
		Episodes: []spindle.EpisodeStatus{{
			RippedPath:    "/r.mkv",
			EncodedPath:   "/e.mkv",
			SubtitledPath: "/s.mkv",
			FinalPath:     "/f.mkv",
		}},
	}
	if got := m.describeItemFileStates(item); got != "RIP ENC SUB FIN" {
		t.Fatalf("describeItemFileStates() = %q, want %q", got, "RIP ENC SUB FIN")
	}
}
