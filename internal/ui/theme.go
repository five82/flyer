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

// themeOrder defines the order for theme cycling.
var themeOrder = []string{"Dracula", "Slate"}

// themes is a registry of available theme constructors.
var themes = map[string]func() Theme{
	"Dracula": draculaTheme,
	"Slate":   slateTheme,
}

// GetTheme returns a theme by name. If the name is not found, returns the default theme.
func GetTheme(name string) Theme {
	if fn, ok := themes[name]; ok {
		return fn()
	}
	return draculaTheme()
}

// ThemeNames returns the list of available theme names in cycling order.
func ThemeNames() []string {
	return themeOrder
}

// NextTheme returns the name of the next theme after the given one.
func NextTheme(current string) string {
	for i, name := range themeOrder {
		if name == current {
			return themeOrder[(i+1)%len(themeOrder)]
		}
	}
	return themeOrder[0]
}

// draculaTheme returns the Dracula dark theme.
// Official palette: https://draculatheme.com/spec
// UI colors derived from dracula/visual-studio-code canonical implementation:
//   - BGDarker #191A21: outermost background, status areas
//   - BGDark #21222C: sidebar, secondary surfaces
//   - Background #282A36: editor, main content panels
//   - BGLight #343746: activity bar, focus states
//   - Selection #44475A: list/text selection
func draculaTheme() Theme {
	return Theme{
		Name: "Dracula",
		Base: BasePalette{
			Background: "#191A21", // BGDarker (outermost, darkest)
			Surface:    "#282A36", // Background (main content panels)
			SurfaceAlt: "#21222C", // BGDark (secondary surfaces)
			FocusBg:    "#2d303d", // Subtle focus (between Background and BGLight)
		},
		Border: BorderPalette{
			Default: "#44475A", // Selection
			Muted:   "#21222C", // BGDark
			Focus:   "#BD93F9", // Purple
			Danger:  "#FF5555", // Red
		},
		Text: TextPalette{
			Heading:    "#F8F8F2", // Foreground
			Primary:    "#F8F8F2", // Foreground
			Secondary:  "#F8F8F2", // Foreground (per spec, main text is Foreground)
			Muted:      "#6272A4", // Comment
			Faint:      "#44475A", // Selection (dimmest readable)
			Accent:     "#BD93F9", // Purple
			AccentSoft: "#FF79C6", // Pink
			Success:    "#50FA7B", // Green
			Warning:    "#FFB86C", // Orange
			Danger:     "#FF5555", // Red
			Info:       "#8BE9FD", // Cyan
		},
		Table: TablePalette{
			HeaderBg:         "#21222C", // BGDark (like inactive tabs)
			HeaderText:       "#F8F8F2", // Foreground
			SelectionBg:      "#44475A", // Selection
			SelectionText:    "#F8F8F2", // Foreground
			Border:           "#44475A", // Selection
			BorderFocus:      "#BD93F9", // Purple
			ProblemHeaderBg:  "#21222C", // BGDark
			ProblemHeaderTxt: "#FFB86C", // Orange
		},
		Problems: ProblemPalette{
			Border:         "#FFB86C", // Orange
			Shortcut:       "#F1FA8C", // Yellow
			Warning:        "#FFB86C", // Orange
			Danger:         "#FF5555", // Red
			SummaryPrimary: "#F8F8F2", // Foreground
			SummaryMuted:   "#6272A4", // Comment
			BarBackground:  "#191A21", // BGDarker
			BarText:        "#FF79C6", // Pink
		},
		Search: SearchPalette{
			Prompt:             "#8BE9FD", // Cyan
			Match:              "#F8F8F2", // Foreground
			Count:              "#F1FA8C", // Yellow
			Hint:               "#6272A4", // Comment
			Error:              "#FF5555", // Red
			HighlightActiveFg:  "#191A21", // BGDarker
			HighlightActiveBg:  "#F1FA8C", // Yellow
			HighlightPassiveFg: "#FFB86C", // Orange
		},
		Badges: BadgePalette{
			Review:   "#FFB86C", // Orange
			Error:    "#FF5555", // Red
			Log:      "#8BE9FD", // Cyan
			Info:     "#50FA7B", // Green
			Fallback: "#BD93F9", // Purple
		},
		StatusColors: map[string]string{
			"pending":             "#6272A4", // Comment (muted)
			"identifying":         "#8BE9FD", // Cyan (active)
			"identified":          "#6272A4", // Comment (completed)
			"ripping":             "#BD93F9", // Purple (active)
			"ripped":              "#6272A4", // Comment (completed)
			"episode_identifying": "#8BE9FD", // Cyan
			"episode_identified":  "#6272A4", // Comment
			"encoding":            "#FF79C6", // Pink (active)
			"encoded":             "#50FA7B", // Green (success)
			"subtitling":          "#8BE9FD", // Cyan (active)
			"subtitled":           "#50FA7B", // Green (success)
			"organizing":          "#FFB86C", // Orange
			"completed":           "#50FA7B", // Green (success)
			"failed":              "#FF5555", // Red (error)
			"review":              "#FFB86C", // Orange (attention)
		},
		LaneColors: map[string]string{
			"foreground": "#8BE9FD", // Cyan
			"background": "#6272A4", // Comment
			"attention":  "#F1FA8C", // Yellow
		},
	}
}

// slateTheme returns the Slate dark theme.
// Based on Tailwind CSS Slate/Sky palette: https://tailwindcss.com/docs/colors
func slateTheme() Theme {
	return Theme{
		Name: "Slate",
		Base: BasePalette{
			Background: "#020617", // slate-950
			Surface:    "#0f172a", // slate-900
			SurfaceAlt: "#1e293b", // slate-800
			FocusBg:    "#283548", // between slate-800 and slate-700
		},
		Border: BorderPalette{
			Default: "#334155", // slate-700
			Muted:   "#1e293b", // slate-800
			Focus:   "#38bdf8", // sky-400
			Danger:  "#f87171", // red-400
		},
		Text: TextPalette{
			Heading:    "#f8fafc", // slate-50
			Primary:    "#f1f5f9", // slate-100
			Secondary:  "#cbd5e1", // slate-300
			Muted:      "#94a3b8", // slate-400
			Faint:      "#64748b", // slate-500
			Accent:     "#38bdf8", // sky-400
			AccentSoft: "#0ea5e9", // sky-500
			Success:    "#22c55e", // green-500
			Warning:    "#f59e0b", // amber-500
			Danger:     "#ef4444", // red-500
			Info:       "#06b6d4", // cyan-500
		},
		Table: TablePalette{
			HeaderBg:         "#1e293b", // slate-800
			HeaderText:       "#f1f5f9", // slate-100
			SelectionBg:      "#0284c7", // sky-600
			SelectionText:    "#f8fafc", // slate-50
			Border:           "#334155", // slate-700
			BorderFocus:      "#38bdf8", // sky-400
			ProblemHeaderBg:  "#1e293b", // slate-800
			ProblemHeaderTxt: "#fde68a", // amber-200
		},
		Problems: ProblemPalette{
			Border:         "#f97316", // orange-500
			Shortcut:       "#fbbf24", // amber-400
			Warning:        "#f59e0b", // amber-500
			Danger:         "#f87171", // red-400
			SummaryPrimary: "#f8fafc", // slate-50
			SummaryMuted:   "#94a3b8", // slate-400
			BarBackground:  "#0f172a", // slate-900
			BarText:        "#fb7185", // rose-400
		},
		Search: SearchPalette{
			Prompt:             "#7dd3fc", // sky-300
			Match:              "#e2e8f0", // slate-200
			Count:              "#fbbf24", // amber-400
			Hint:               "#94a3b8", // slate-400
			Error:              "#f87171", // red-400
			HighlightActiveFg:  "#020617", // slate-950
			HighlightActiveBg:  "#facc15", // yellow-400
			HighlightPassiveFg: "#f97316", // orange-500
		},
		Badges: BadgePalette{
			Review:   "#fbbf24", // amber-400
			Error:    "#f87171", // red-400
			Log:      "#38bdf8", // sky-400
			Info:     "#5eead4", // teal-300
			Fallback: "#a78bfa", // violet-400
		},
		StatusColors: map[string]string{
			"pending":             "#64748b", // slate-500 (muted)
			"identifying":         "#38bdf8", // sky-400 (active)
			"identified":          "#0284c7", // sky-600 (completed)
			"ripping":             "#8b5cf6", // violet-500 (active)
			"ripped":              "#6366f1", // indigo-500 (completed)
			"episode_identifying": "#a78bfa", // violet-400
			"episode_identified":  "#8b5cf6", // violet-500
			"encoding":            "#ec4899", // pink-500 (active)
			"encoded":             "#22c55e", // green-500 (success)
			"subtitling":          "#06b6d4", // cyan-500 (active)
			"subtitled":           "#14b8a6", // teal-500 (success)
			"organizing":          "#f59e0b", // amber-500
			"completed":           "#16a34a", // green-600 (success)
			"failed":              "#dc2626", // red-600 (error)
			"review":              "#ea580c", // orange-600 (attention)
		},
		LaneColors: map[string]string{
			"foreground": "#7dd3fc", // sky-300
			"background": "#64748b", // slate-500
			"attention":  "#fbbf24", // amber-400
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

func (t Theme) TableSelectionTextHex() string {
	return t.Table.SelectionText
}

func (t Theme) TableBorderColor() tcell.Color {
	return hexToColor(t.Table.Border)
}

func (t Theme) TableBorderFocusColor() tcell.Color {
	return hexToColor(t.Table.BorderFocus)
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
