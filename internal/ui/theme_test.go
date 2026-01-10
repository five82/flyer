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

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	if len(names) != 2 {
		t.Fatalf("ThemeNames() returned %d names, want 2", len(names))
	}
	if names[0] != "Dracula" || names[1] != "Slate" {
		t.Fatalf("ThemeNames() = %v, want [Dracula Slate]", names)
	}
}

func TestNextTheme(t *testing.T) {
	if got := NextTheme("Dracula"); got != "Slate" {
		t.Fatalf("NextTheme(Dracula) = %q, want Slate", got)
	}
	if got := NextTheme("Slate"); got != "Dracula" {
		t.Fatalf("NextTheme(Slate) = %q, want Dracula", got)
	}
	if got := NextTheme("Unknown"); got != "Dracula" {
		t.Fatalf("NextTheme(Unknown) = %q, want Dracula", got)
	}
}

func TestGetTheme(t *testing.T) {
	dracula := GetTheme("Dracula")
	if dracula.Name != "Dracula" {
		t.Fatalf("GetTheme(Dracula).Name = %q, want Dracula", dracula.Name)
	}

	slate := GetTheme("Slate")
	if slate.Name != "Slate" {
		t.Fatalf("GetTheme(Slate).Name = %q, want Slate", slate.Name)
	}

	unknown := GetTheme("Unknown")
	if unknown.Name != "Dracula" {
		t.Fatalf("GetTheme(Unknown).Name = %q, want Dracula (fallback)", unknown.Name)
	}
}

func TestDefaultThemeIsDracula(t *testing.T) {
	th := defaultTheme()
	if th.Name != "Dracula" {
		t.Fatalf("defaultTheme().Name = %q, want Dracula", th.Name)
	}
}
