package spindle

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEncodingHelpers(t *testing.T) {
	if (*EncodingStatus)(nil).ETADuration() != 0 {
		t.Fatalf("ETADuration on nil should be 0")
	}
	if (&EncodingStatus{ETASeconds: 2.5}).ETADuration() != 2500*time.Millisecond {
		t.Fatalf("ETADuration mismatch")
	}
	if (&EncodingStatus{TotalFrames: 0, CurrentFrame: 10}).FramePercent() != 0 {
		t.Fatalf("FramePercent should be 0 when TotalFrames<=0")
	}
	if got := (&EncodingStatus{TotalFrames: 100, CurrentFrame: 25}).FramePercent(); got != 0.25 {
		t.Fatalf("FramePercent = %v, want 0.25", got)
	}
}

func TestQueueItemHelpers(t *testing.T) {
	item := QueueItem{DraptoPreset: " default "}
	if item.DraptoPresetLabel() != "Default" {
		t.Fatalf("DraptoPresetLabel = %q, want Default", item.DraptoPresetLabel())
	}
	item.DraptoPreset = "GrAiN"
	if item.DraptoPresetLabel() != "Grain" {
		t.Fatalf("DraptoPresetLabel = %q, want Grain", item.DraptoPresetLabel())
	}
	item.DraptoPreset = "custom"
	if item.DraptoPresetLabel() != "Custom" {
		t.Fatalf("DraptoPresetLabel = %q, want Custom", item.DraptoPresetLabel())
	}
}

func TestParseTimeLayouts(t *testing.T) {
	rfc := "2025-12-13T10:11:12Z"
	if parseTime(rfc).IsZero() {
		t.Fatalf("parseTime should parse RFC3339")
	}
	custom := "2025-12-13 10:11:12"
	got := parseTime(custom)
	if got.IsZero() {
		t.Fatalf("parseTime should parse spindle timestamp")
	}
	if got.Year() != 2025 || got.Month() != time.December || got.Day() != 13 {
		t.Fatalf("parseTime = %v, want 2025-12-13", got)
	}
}

func TestParseRipSpec(t *testing.T) {
	item := QueueItem{}
	sum, err := item.ParseRipSpec()
	if err != nil {
		t.Fatalf("ParseRipSpec returned error: %v", err)
	}
	if sum.ContentKey != "" || sum.Metadata != nil || len(sum.Titles) != 0 || len(sum.Episodes) != 0 {
		t.Fatalf("ParseRipSpec = %#v, want empty summary", sum)
	}

	item.RipSpec = json.RawMessage(`{"content_key":"x"}`)
	sum, err = item.ParseRipSpec()
	if err != nil {
		t.Fatalf("ParseRipSpec returned error: %v", err)
	}
	if sum.ContentKey != "x" {
		t.Fatalf("ContentKey = %q, want x", sum.ContentKey)
	}

	item.RipSpec = json.RawMessage("{not-json")
	if _, err := item.ParseRipSpec(); err == nil {
		t.Fatalf("ParseRipSpec returned nil error, want error")
	}
}

func TestEpisodeSnapshot_UsesAPIFieldsWhenPresent(t *testing.T) {
	item := QueueItem{
		Episodes: []EpisodeStatus{
			{Key: "a", RippedPath: "/r"},
			{Key: "b", EncodedPath: "/e"},
		},
	}
	episodes, totals := item.EpisodeSnapshot()
	if len(episodes) != 2 {
		t.Fatalf("episodes len = %d, want 2", len(episodes))
	}
	if totals.Planned != 2 || totals.Ripped != 1 || totals.Encoded != 1 || totals.Final != 0 {
		t.Fatalf("totals = %#v, want planned=2 ripped=1 encoded=1 final=0", totals)
	}
	episodes[0].Key = "mutated"
	episodes2, _ := item.EpisodeSnapshot()
	if episodes2[0].Key != "a" {
		t.Fatalf("EpisodeSnapshot should return a copy of episodes")
	}
}

func TestEpisodeSnapshot_DerivesFromRipSpec(t *testing.T) {
	raw := json.RawMessage(`{
  "titles": [
    {"id": 1, "name": "Show", "episode_title": "Pilot", "duration": 1800},
    {"id": 2, "name": "Show", "episode_title": "Second", "duration": 1790}
  ],
  "episodes": [
    {"key": "S01E02", "title_id": 2, "season": 1, "episode": 2, "episode_title": "", "runtime_seconds": 0, "output_basename": "ep2"},
    {"key": "S01E01", "title_id": 1, "season": 1, "episode": 1, "episode_title": "", "runtime_seconds": 0, "output_basename": "ep1"}
  ],
  "assets": {
    "ripped": [{"episode_key": "S01E01", "path": "/ripped1.mkv"}],
    "encoded": [{"episode_key": "S01E02", "path": "/encoded2.mkv"}],
    "final": [{"episode_key": "S01E02", "path": "/final2.mkv"}]
  }
}`)

	item := QueueItem{RipSpec: raw}
	episodes, totals := item.EpisodeSnapshot()
	if len(episodes) != 2 {
		t.Fatalf("episodes len = %d, want 2", len(episodes))
	}
	if episodes[0].Episode != 1 || episodes[1].Episode != 2 {
		t.Fatalf("episodes not sorted: %#v", episodes)
	}
	if episodes[0].Stage != "ripped" || episodes[0].RippedPath != "/ripped1.mkv" {
		t.Fatalf("episode1 = %#v, want ripped", episodes[0])
	}
	if episodes[1].Stage != "final" || episodes[1].FinalPath != "/final2.mkv" {
		t.Fatalf("episode2 = %#v, want final", episodes[1])
	}
	if totals.Planned != 2 || totals.Ripped != 1 || totals.Encoded != 1 || totals.Final != 1 {
		t.Fatalf("totals = %#v, want planned=2 ripped=1 encoded=1 final=1", totals)
	}
}
