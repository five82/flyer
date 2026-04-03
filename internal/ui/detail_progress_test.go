package ui

import (
	"testing"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

func TestCountEpisodesForPipelineStage_EpisodeIdentifiedUsesMappedFields(t *testing.T) {
	episodes := []spindle.EpisodeStatus{
		{Key: "s02_001", Stage: "encoded", Episode: 29, MatchScore: 0.94},
		{Key: "s02_002", Stage: "encoded", Episode: 28, MatchScore: 0.92},
		{Key: "s02_003", Stage: "encoded", Episode: 27, MatchScore: 0.93},
		{Key: "s02_004", Stage: "ripped", Episode: 26, MatchScore: 0.94},
		{Key: "s02_005", Stage: "ripped", Episode: 25, MatchScore: 0.94},
		{Key: "s02_006", Stage: "ripped", Episode: 30, MatchScore: 0.96},
	}

	gotMatched := countEpisodesForPipelineStage("episode_identified", "episode_identified", episodes, 6, 0, "encoded", 6)
	if gotMatched != 6 {
		t.Fatalf("episode_identified count = %d, want 6", gotMatched)
	}

	gotEncoded := countEpisodesForPipelineStage("encoded", "encoded", episodes, 6, 0, "encoded", 6)
	if gotEncoded != 3 {
		t.Fatalf("encoded count = %d, want 3", gotEncoded)
	}
}

func TestCountEpisodesForPipelineStage_ItemLevelStagesUseGlobalProgression(t *testing.T) {
	episodes := []spindle.EpisodeStatus{
		{Key: "s02_001", Stage: "ripped"},
		{Key: "s02_002", Stage: "planned"},
		{Key: "s02_003", Stage: "planned"},
	}

	gotIdentification := countEpisodesForPipelineStage("identifying", "identified", episodes, 3, 0, "ripped", 3)
	if gotIdentification != 3 {
		t.Fatalf("identifying count = %d, want 3 after item advanced to ripping", gotIdentification)
	}

	gotAudio := countEpisodesForPipelineStage("audio_analyzed", "audio_analyzed", episodes, 3, 0, "subtitled", 3)
	if gotAudio != 3 {
		t.Fatalf("audio_analyzed count = %d, want 3 after item advanced past analysis", gotAudio)
	}
}

func TestCountEpisodesForPipelineStage_EpisodeIdentifiedPrefersExplicitCount(t *testing.T) {
	episodes := []spindle.EpisodeStatus{
		{Key: "s02_001", Stage: "encoded", Episode: 29, MatchScore: 0.94},
		{Key: "s02_002", Stage: "encoded", Episode: 28, MatchScore: 0.92},
		{Key: "s02_003", Stage: "encoded", Episode: 27, MatchScore: 0.93},
		{Key: "s02_004", Stage: "ripped", Episode: 26, MatchScore: 0.94},
		{Key: "s02_005", Stage: "ripped", Episode: 25, MatchScore: 0.94},
		{Key: "s02_006", Stage: "ripped", Episode: 30, MatchScore: 0.96},
	}

	got := countEpisodesForPipelineStage("episode_identified", "episode_identified", episodes, 6, 3, "encoded", 6)
	if got != 3 {
		t.Fatalf("episode_identified count = %d, want 3", got)
	}
}

func TestIsEpisodeMapped(t *testing.T) {
	cases := []struct {
		name string
		ep   spindle.EpisodeStatus
		want bool
	}{
		{name: "placeholder", ep: spindle.EpisodeStatus{Episode: 0, MatchScore: 0, MatchedEpisode: 0}, want: false},
		{name: "resolved episode number", ep: spindle.EpisodeStatus{Episode: 26}, want: true},
		{name: "resolved by match score", ep: spindle.EpisodeStatus{Episode: 0, MatchScore: 0.91}, want: true},
		{name: "resolved by matched episode", ep: spindle.EpisodeStatus{Episode: 0, MatchedEpisode: 26}, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isEpisodeMapped(tc.ep)
			if got != tc.want {
				t.Fatalf("isEpisodeMapped() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEstimateETA_SuppressedDuringAnalyzing(t *testing.T) {
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	m.stageFirstSeen[1] = stageObservation{
		stage:     "analyzing",
		firstSeen: time.Now().Add(-10 * time.Minute),
	}
	item := spindle.QueueItem{
		ID:       1,
		Progress: spindle.QueueProgress{Stage: "Analyzing", Percent: 50},
	}
	if got := m.estimateETA(item); got != "" {
		t.Fatalf("expected empty ETA during Analyzing, got %q", got)
	}
}

func TestEstimateETA_SuppressedBelowThreshold(t *testing.T) {
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	m.stageFirstSeen[1] = stageObservation{
		stage:     "ripping",
		firstSeen: time.Now().Add(-5 * time.Minute),
	}
	item := spindle.QueueItem{
		ID:       1,
		Progress: spindle.QueueProgress{Stage: "Ripping", Percent: 4},
	}
	if got := m.estimateETA(item); got != "" {
		t.Fatalf("expected empty ETA at 4%%, got %q", got)
	}
}

func TestEstimateETA_UsesStageEntryTime(t *testing.T) {
	// Stage started 10 minutes ago, item was created 2 hours ago.
	// With percent at 50%, ETA should be ~10m (based on stage time),
	// NOT ~2h (based on creation time).
	stageStart := time.Now().Add(-10 * time.Minute)
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	m.stageFirstSeen[1] = stageObservation{
		stage:     "ripping",
		firstSeen: stageStart,
	}
	item := spindle.QueueItem{
		ID:        1,
		CreatedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		Progress:  spindle.QueueProgress{Stage: "Ripping", Percent: 50},
	}
	got := m.estimateETA(item)
	if got == "" {
		t.Fatal("expected non-empty ETA at 50%")
	}
	// The ETA should be roughly 10m (stage elapsed 10m at 50% → 10m remaining).
	// Definitely should NOT be ~2h. Check it contains "10m" or similar.
	// Since formatDuration rounds, just ensure it doesn't say hours.
	if got == "" {
		t.Fatal("expected ETA string")
	}
	// Sanity: ETA should be short, not contain "h" (which would indicate
	// it incorrectly used creation time 2h ago).
	for _, ch := range got {
		if ch == 'h' {
			t.Fatalf("ETA %q looks too large; stage-based ETA should be ~10m, not hours", got)
		}
	}
}

func TestEstimateETA_EncodingSingleFilePrefersEncoderETA(t *testing.T) {
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	m.stageFirstSeen[1] = stageObservation{
		stage:     "encoding",
		firstSeen: time.Now().Add(-10 * time.Minute),
	}
	item := spindle.QueueItem{
		ID:       1,
		Progress: spindle.QueueProgress{Stage: "encoding", Percent: 50},
		Encoding: &spindle.EncodingStatus{ETASeconds: 90},
	}
	got := m.estimateETA(item)
	if got == "" {
		t.Fatal("expected encoding ETA")
	}
	if got != "ETA 1m 30s" {
		t.Fatalf("estimateETA() = %q, want %q", got, "ETA 1m 30s")
	}
}

func TestEstimateETA_EncodingMultiFileIgnoresPerFileETA(t *testing.T) {
	m := Model{stageFirstSeen: make(map[int64]stageObservation)}
	m.stageFirstSeen[1] = stageObservation{
		stage:     "encoding",
		firstSeen: time.Now().Add(-10 * time.Minute),
	}
	item := spindle.QueueItem{
		ID:       1,
		Progress: spindle.QueueProgress{Stage: "encoding", Percent: 50},
		Encoding: &spindle.EncodingStatus{ETASeconds: 90},
		Episodes: []spindle.EpisodeStatus{{Key: "s01e01"}, {Key: "s01e02"}},
	}
	got := m.estimateETA(item)
	if got == "" {
		t.Fatal("expected aggregate encoding ETA")
	}
	if got == "ETA 1m 30s" {
		t.Fatalf("estimateETA() = %q, want aggregate stage ETA instead of per-file ETA", got)
	}
	for _, ch := range got {
		if ch == 'h' {
			t.Fatalf("ETA %q looks too large; stage-based ETA should be minutes, not hours", got)
		}
	}
}
