package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

type Theme struct {
	Name         string
	Base         BasePalette
	Border       BorderPalette
	Text         TextPalette
	Table        TablePalette
	Problems     ProblemPalette
	Search       SearchPalette
	Badges       BadgePalette
	StatusColors map[string]string
	LaneColors   map[string]string
}

type BasePalette struct {
	Background string
	Surface    string
	SurfaceAlt string
}

type BorderPalette struct {
	Default string
	Muted   string
	Focus   string
	Danger  string
}

type TextPalette struct {
	Heading    string
	Primary    string
	Secondary  string
	Muted      string
	Faint      string
	Accent     string
	AccentSoft string
	Success    string
	Warning    string
	Danger     string
	Info       string
}

type TablePalette struct {
	HeaderBg         string
	HeaderText       string
	SelectionBg      string
	SelectionText    string
	Border           string
	BorderFocus      string
	ProblemHeaderBg  string
	ProblemHeaderTxt string
}

type ProblemPalette struct {
	Border         string
	Shortcut       string
	Warning        string
	Danger         string
	SummaryPrimary string
	SummaryMuted   string
	BarBackground  string
	BarText        string
}

type BadgePalette struct {
	Review string
	Error  string
	Log    string
	Info   string
}

type SearchPalette struct {
	Prompt             string
	Match              string
	Count              string
	Hint               string
	Error              string
	HighlightActiveFg  string
	HighlightActiveBg  string
	HighlightPassiveFg string
}

func defaultTheme() Theme {
	return Theme{
		Name: "CarbonNight",
		Base: BasePalette{
			Background: "#05070f",
			Surface:    "#0f172a",
			SurfaceAlt: "#172033",
		},
		Border: BorderPalette{
			Default: "#1f2937",
			Muted:   "#111827",
			Focus:   "#38bdf8",
			Danger:  "#f87171",
		},
		Text: TextPalette{
			Heading:    "#f8fafc",
			Primary:    "#e2e8f0",
			Secondary:  "#cbd5f5",
			Muted:      "#94a3b8",
			Faint:      "#64748b",
			Accent:     "#7dd3fc",
			AccentSoft: "#38bdf8",
			Success:    "#4ade80",
			Warning:    "#fbbf24",
			Danger:     "#f87171",
			Info:       "#5eead4",
		},
		Table: TablePalette{
			HeaderBg:         "#1e293b",
			HeaderText:       "#f1f5f9",
			SelectionBg:      "#1d4ed8",
			SelectionText:    "#f8fafc",
			Border:           "#273449",
			BorderFocus:      "#38bdf8",
			ProblemHeaderBg:  "#2d1f1f",
			ProblemHeaderTxt: "#fde68a",
		},
		Problems: ProblemPalette{
			Border:         "#f97316",
			Shortcut:       "#fbbf24",
			Warning:        "#f59e0b",
			Danger:         "#f87171",
			SummaryPrimary: "#f8fafc",
			SummaryMuted:   "#94a3b8",
			BarBackground:  "#1a0f0f",
			BarText:        "#fb7185",
		},
		Search: SearchPalette{
			Prompt:             "#7dd3fc",
			Match:              "#e2e8f0",
			Count:              "#fbbf24",
			Hint:               "#94a3b8",
			Error:              "#f87171",
			HighlightActiveFg:  "#05070f",
			HighlightActiveBg:  "#facc15",
			HighlightPassiveFg: "#f97316",
		},
		Badges: BadgePalette{
			Review: "#fbbf24",
			Error:  "#f87171",
			Log:    "#38bdf8",
			Info:   "#5eead4",
		},
		StatusColors: map[string]string{
			"pending":     "#94a3b8",
			"identifying": "#7dd3fc",
			"identified":  "#7dd3fc",
			"ripping":     "#818cf8",
			"ripped":      "#a78bfa",
			"encoding":    "#f472b6",
			"encoded":     "#34d399",
			"organizing":  "#2dd4bf",
			"completed":   "#4ade80",
			"failed":      "#f87171",
			"review":      "#fbbf24",
		},
		LaneColors: map[string]string{
			"foreground": "#7dd3fc",
			"background": "#64748b",
			"attention":  "#fbbf24",
		},
	}
}

func (t Theme) BackgroundColor() tcell.Color {
	return hexToColor(t.Base.Background)
}

func (t Theme) SurfaceColor() tcell.Color {
	return hexToColor(t.Base.Surface)
}

func (t Theme) SurfaceAltColor() tcell.Color {
	return hexToColor(t.Base.SurfaceAlt)
}

func (t Theme) BorderColor() tcell.Color {
	return hexToColor(t.Border.Default)
}

func (t Theme) BorderMutedColor() tcell.Color {
	return hexToColor(t.Border.Muted)
}

func (t Theme) BorderFocusColor() tcell.Color {
	return hexToColor(t.Border.Focus)
}

func (t Theme) BorderDangerColor() tcell.Color {
	return hexToColor(t.Border.Danger)
}

func (t Theme) TableHeaderBackground() tcell.Color {
	return hexToColor(t.Table.HeaderBg)
}

func (t Theme) TableHeaderTextColor() tcell.Color {
	return hexToColor(t.Table.HeaderText)
}

func (t Theme) TableSelectionBackground() tcell.Color {
	return hexToColor(t.Table.SelectionBg)
}

func (t Theme) TableSelectionTextColor() tcell.Color {
	return hexToColor(t.Table.SelectionText)
}

func (t Theme) TableBorderColor() tcell.Color {
	if strings.TrimSpace(t.Table.Border) != "" {
		return hexToColor(t.Table.Border)
	}
	return t.BorderColor()
}

func (t Theme) TableBorderFocusColor() tcell.Color {
	if strings.TrimSpace(t.Table.BorderFocus) != "" {
		return hexToColor(t.Table.BorderFocus)
	}
	return t.BorderFocusColor()
}

func (t Theme) ProblemBorderColor() tcell.Color {
	return hexToColor(t.Problems.Border)
}

func (t Theme) ProblemHeaderBackground() tcell.Color {
	return hexToColor(t.Table.ProblemHeaderBg)
}

func (t Theme) ProblemHeaderTextColor() tcell.Color {
	return hexToColor(t.Table.ProblemHeaderTxt)
}

func (t Theme) StatusColor(status string) string {
	key := strings.ToLower(strings.TrimSpace(status))
	if c, ok := t.StatusColors[key]; ok {
		return c
	}
	return t.Text.Secondary
}

func (t Theme) LaneColor(lane string) string {
	key := strings.ToLower(strings.TrimSpace(lane))
	if c, ok := t.LaneColors[key]; ok {
		return c
	}
	return t.Text.Faint
}

func (t Theme) BadgeColor(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "review":
		return t.Badges.Review
	case "error":
		return t.Badges.Error
	case "log":
		return t.Badges.Log
	case "info":
		return t.Badges.Info
	default:
		return t.Text.Secondary
	}
}

func hexToColor(hex string) tcell.Color {
	if strings.TrimSpace(hex) == "" {
		return tcell.ColorDefault
	}
	return tcell.GetColor(hex)
}
