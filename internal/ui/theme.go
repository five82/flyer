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
	FocusBg    string
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
	Review   string
	Error    string
	Log      string
	Info     string
	Fallback string
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

// themes is a registry of available theme constructors.
var themes = map[string]func() Theme{
	"CarbonNight": carbonNightTheme,
}

// GetTheme returns a theme by name. If the name is not found, returns the default theme.
func GetTheme(name string) Theme {
	if fn, ok := themes[name]; ok {
		return fn()
	}
	return carbonNightTheme()
}

// defaultTheme returns the default theme (CarbonNight).
func defaultTheme() Theme {
	return carbonNightTheme()
}

// carbonNightTheme returns the CarbonNight dark theme.
func carbonNightTheme() Theme {
	return Theme{
		Name: "CarbonNight",
		Base: BasePalette{
			Background: "#05070f",
			Surface:    "#0f172a",
			SurfaceAlt: "#172033",
			FocusBg:    "#1a2942", // Slightly brighter than SurfaceAlt for focus indication
		},
		Border: BorderPalette{
			Default: "#1f2937",
			Muted:   "#111827",
			Focus:   "#38bdf8",
			Danger:  "#f87171",
		},
		Text: TextPalette{
			Heading:    "#f8fafc", // Keep white for headings
			Primary:    "#f1f5f9", // Brighter primary for better readability
			Secondary:  "#cbd5f5", // Good secondary, keep as is
			Muted:      "#94a3b8", // Good muted, keep as is
			Faint:      "#64748b", // Good faint, keep as is
			Accent:     "#38bdf8", // Brighter accent for better visibility
			AccentSoft: "#0ea5e9", // Brighter soft accent
			Success:    "#22c55e", // Brighter success green
			Warning:    "#f59e0b", // Brighter warning amber
			Danger:     "#ef4444", // Brighter danger red
			Info:       "#06b6d4", // Brighter info cyan
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
			Review:   "#fbbf24",
			Error:    "#f87171",
			Log:      "#38bdf8",
			Info:     "#5eead4",
			Fallback: "#a78bfa",
		},
		StatusColors: map[string]string{
			"pending":             "#64748b", // Muted gray for waiting state
			"identifying":         "#3b82f6", // Bright blue for active identification
			"identified":          "#1d4ed8", // Deeper blue for completed identification
			"ripping":             "#8b5cf6", // Vibrant purple for active ripping
			"ripped":              "#0ea5e9", // Blue-cyan for ripped completion (distinct from ripping WORK)
			"episode_identifying": "#a855f7", // Light purple for episode identification
			"episode_identified":  "#9333ea", // Medium purple for episode identified
			"encoding":            "#ec4899", // Bright pink for active encoding
			"encoded":             "#10b981", // Emerald green for encoded completion
			"subtitling":          "#06b6d4", // Cyan for active subtitling
			"subtitled":           "#14b8a6", // Teal for subtitle completion
			"organizing":          "#f59e0b", // Amber for organizing phase
			"completed":           "#16a34a", // Deep green for final completion
			"failed":              "#dc2626", // Strong red for failures
			"review":              "#ea580c", // Dark orange for review state
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

func (t Theme) FocusBackgroundColor() tcell.Color {
	return hexToColor(t.Base.FocusBg)
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
	case "fallback":
		return t.Badges.Fallback
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
