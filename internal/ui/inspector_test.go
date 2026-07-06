package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

func inspectorModelFor(item spindle.QueueItem) Model {
	m := New(Options{ThemeName: "slate"})
	m.width = 120
	m.snapshot.Queue = []spindle.QueueItem{item}
	m.inspectedID = item.ID
	return m
}

func TestInspectorTabBar_ProblemsMarkerFollowsAttention(t *testing.T) {
	m := inspectorModelFor(spindle.QueueItem{ID: 9, Stage: "failed", ErrorMessage: "boom"})
	got := stripANSI(m.renderInspectorTabBar(m.theme.BandStyles()))
	if !strings.Contains(got, "Problems ⚠") {
		t.Fatalf("problem item must mark the Problems tab, got %q", got)
	}

	m = inspectorModelFor(spindle.QueueItem{ID: 9, Stage: "encoding"})
	got = stripANSI(m.renderInspectorTabBar(m.theme.BandStyles()))
	if strings.Contains(got, "⚠") {
		t.Fatalf("healthy item must not mark the Problems tab, got %q", got)
	}
}

func TestInspectorItemLine_IdentitySegments(t *testing.T) {
	item := spindle.QueueItem{
		ID:           9,
		Stage:        "encoding",
		DisplayTitle: "The Abyss",
		Metadata:     json.RawMessage(`{"year":"1989"}`),
		Source:       &spindle.SourceTitle{TitleID: 2, DurationSeconds: 171 * 60},
	}
	m := inspectorModelFor(item)
	got := stripANSI(m.renderInspectorItemLine(m.theme.BandStyles()))
	for _, want := range []string{"The Abyss", "(1989)", "2h 51m", "#9"} {
		if !strings.Contains(got, want) {
			t.Fatalf("item line missing %q, got %q", want, got)
		}
	}
}

func TestInspectorItemLine_NoDuplicateYear(t *testing.T) {
	item := spindle.QueueItem{
		ID:           9,
		Stage:        "encoding",
		DisplayTitle: "The Abyss (1989)",
		Metadata:     json.RawMessage(`{"year":"1989"}`),
	}
	m := inspectorModelFor(item)
	got := stripANSI(m.renderInspectorItemLine(m.theme.BandStyles()))
	if strings.Count(got, "1989") != 1 {
		t.Fatalf("year must not repeat when the title carries it, got %q", got)
	}
}

func TestInspectorItemLine_ShedsRuntimeFirstWhenNarrow(t *testing.T) {
	item := spindle.QueueItem{
		ID:           9,
		Stage:        "encoding",
		DisplayTitle: "A Fairly Long Movie Title For Shedding",
		Metadata:     json.RawMessage(`{"year":"1989"}`),
		Source:       &spindle.SourceTitle{TitleID: 2, DurationSeconds: 171 * 60},
	}
	m := inspectorModelFor(item)
	wide := stripANSI(m.renderInspectorItemLine(m.theme.BandStyles()))
	if !strings.Contains(wide, "2h 51m") {
		t.Fatalf("wide item line missing runtime, got %q", wide)
	}

	m.width = len("Queue › " + item.DisplayTitle + "  (1989)  ENCODING  #9")
	narrow := stripANSI(m.renderInspectorItemLine(m.theme.BandStyles()))
	if strings.Contains(narrow, "2h 51m") {
		t.Fatalf("narrow item line must shed runtime first, got %q", narrow)
	}
	if !strings.Contains(narrow, item.DisplayTitle) {
		t.Fatalf("narrow item line must keep the title, got %q", narrow)
	}
}

func TestStatusChips_StoppedItem(t *testing.T) {
	m := New(Options{ThemeName: "slate"})
	got := stripANSI(m.renderStatusChips(spindle.QueueItem{ID: 7, Stage: "encoding", UserStopped: true}, m.theme.Styles()))
	if !strings.Contains(got, "STOPPED") {
		t.Fatalf("user-stopped item missing STOPPED chip, got %q", got)
	}
}
