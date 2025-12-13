package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestThemeLookups(t *testing.T) {
	th := defaultTheme()

	if got := th.StatusColor("  failed "); got != th.StatusColors["failed"] {
		t.Fatalf("StatusColor = %q, want %q", got, th.StatusColors["failed"])
	}
	if got := th.StatusColor("unknown"); got != th.Text.Secondary {
		t.Fatalf("StatusColor unknown = %q, want %q", got, th.Text.Secondary)
	}

	if got := th.LaneColor("foreground"); got != th.LaneColors["foreground"] {
		t.Fatalf("LaneColor = %q, want %q", got, th.LaneColors["foreground"])
	}
	if got := th.LaneColor("unknown"); got != th.Text.Faint {
		t.Fatalf("LaneColor unknown = %q, want %q", got, th.Text.Faint)
	}

	if got := th.BadgeColor("review"); got != th.Badges.Review {
		t.Fatalf("BadgeColor(review) = %q, want %q", got, th.Badges.Review)
	}
	if got := th.BadgeColor("other"); got != th.Text.Secondary {
		t.Fatalf("BadgeColor(other) = %q, want %q", got, th.Text.Secondary)
	}
}

func TestHexToColor_EmptyDefaults(t *testing.T) {
	if got := hexToColor(" "); got != tcell.ColorDefault {
		t.Fatalf("hexToColor empty = %v, want %v", got, tcell.ColorDefault)
	}
}

func TestFilterStrings_TrimsEmpty(t *testing.T) {
	values := []string{" a ", " ", "", "\t", "b"}
	out := filterStrings(values)
	if len(out) != 2 || out[0] != " a " || out[1] != "b" {
		t.Fatalf("filterStrings = %#v, want [%q %q]", out, " a ", "b")
	}
}
