package ui

import (
	"strings"
	"testing"

	"github.com/five82/flyer/internal/spindle"
)

func TestQueueDiscColumnForTVItems(t *testing.T) {
	items := []spindle.QueueItem{{
		ID:           7,
		DisplayTitle: "Example Show Season 01",
		DiscNumber:   2,
		Stage:        "encoding",
	}}
	cols := computeQueueColumns(items, 120)
	if cols.disc != len("DISC") {
		t.Fatalf("disc column width = %d, want %d", cols.disc, len("DISC"))
	}

	m := New(Options{ThemeName: "slate"})
	m.width = 120
	styles := m.theme.Styles()
	header := stripANSI(renderQueueHeaderRow(cols, styles))
	if !strings.Contains(header, "DISC") {
		t.Fatalf("queue header missing DISC column: %q", header)
	}
	row := stripANSI(m.renderQueueRow(items[0], cols, false, styles))
	if !strings.Contains(row, "  2   ") {
		t.Fatalf("queue row missing disc number: %q", row)
	}
}

func TestQueueOmitsDiscColumnWithoutDiscNumbers(t *testing.T) {
	items := []spindle.QueueItem{{ID: 8, DisplayTitle: "Example Movie", Stage: "encoding"}}
	cols := computeQueueColumns(items, 120)
	if cols.disc != 0 {
		t.Fatalf("disc column width = %d, want 0", cols.disc)
	}
	if header := stripANSI(renderQueueHeaderRow(cols, New(Options{ThemeName: "slate"}).theme.Styles())); strings.Contains(header, "DISC") {
		t.Fatalf("movie-only queue unexpectedly has DISC column: %q", header)
	}
}
