package tea

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines colors and styles for the UI.
// Colors match tview theme exactly for visual parity.
type Theme struct {
	Name string

	// Base colors (from tview Base palette)
	Background string // Outermost background
	Surface    string // Main content panels
	SurfaceAlt string // Secondary surfaces
	FocusBg    string // Focus/active states

	// Table colors (from tview Table palette)
	SelectionBg   string // Selected row background
	SelectionText string // Selected row text

	// Border colors (from tview Border palette)
	Border      string // Default border
	BorderMuted string // Muted border
	BorderFocus string // Focus border

	// Text colors (from tview Text palette)
	Text    string
	Muted   string
	Faint   string
	Accent  string
	Success string
	Warning string
	Danger  string
	Info    string

	// Status colors
	StatusColors map[string]string
}

// Styles returns Lipgloss styles for this theme.
func (t Theme) Styles() Styles {
	return Styles{
		// Base styles
		Background: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Background)),

		Surface: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Surface)).
			Foreground(lipgloss.Color(t.Text)),

		SurfaceAlt: lipgloss.NewStyle().
			Background(lipgloss.Color(t.SurfaceAlt)).
			Foreground(lipgloss.Color(t.Text)),

		// Text styles
		Text: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Text)),

		MutedText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Muted)),

		FaintText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Faint)),

		AccentText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Accent)),

		SuccessText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Success)).
			Bold(true),

		WarningText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Warning)),

		DangerText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Danger)).
			Bold(true),

		InfoText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Info)),

		// Component styles
		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Surface)).
			Foreground(lipgloss.Color(t.Text)).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Surface)).
			Foreground(lipgloss.Color(t.Muted)).
			Padding(0, 1),

		Logo: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Warning)).
			Bold(true),

		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color(t.SelectionBg)).
			Foreground(lipgloss.Color(t.SelectionText)),

		// Status badge style generator
		statusColors: t.StatusColors,
		background:   t.Background,
	}
}

// Styles contains pre-built Lipgloss styles for the theme.
type Styles struct {
	// Base
	Background lipgloss.Style
	Surface    lipgloss.Style
	SurfaceAlt lipgloss.Style

	// Text
	Text        lipgloss.Style
	MutedText   lipgloss.Style
	FaintText   lipgloss.Style
	AccentText  lipgloss.Style
	SuccessText lipgloss.Style
	WarningText lipgloss.Style
	DangerText  lipgloss.Style
	InfoText    lipgloss.Style

	// Components
	Header   lipgloss.Style
	Footer   lipgloss.Style
	Logo     lipgloss.Style
	Selected lipgloss.Style

	// For dynamic status colors
	statusColors map[string]string
	background   string
}

// StatusStyle returns a style for the given status.
func (s Styles) StatusStyle(status string) lipgloss.Style {
	color := s.statusColors[status]
	if color == "" {
		color = "#6272A4" // Default muted
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.background)).
		Background(lipgloss.Color(color)).
		Padding(0, 1)
}

// WithBackground returns a copy of Styles with all text styles having the specified background.
// This ensures styled text has explicit backgrounds instead of transparent/inherit.
func (s Styles) WithBackground(bgColor string) Styles {
	bg := lipgloss.Color(bgColor)

	return Styles{
		// Base styles with background
		Background: s.Background.Background(bg),
		Surface:    s.Surface.Background(bg),
		SurfaceAlt: s.SurfaceAlt.Background(bg),

		// Text styles with background
		Text:        s.Text.Background(bg),
		MutedText:   s.MutedText.Background(bg),
		FaintText:   s.FaintText.Background(bg),
		AccentText:  s.AccentText.Background(bg),
		SuccessText: s.SuccessText.Background(bg),
		WarningText: s.WarningText.Background(bg),
		DangerText:  s.DangerText.Background(bg),
		InfoText:    s.InfoText.Background(bg),

		// Component styles with background
		Header:   s.Header.Background(bg),
		Footer:   s.Footer.Background(bg),
		Logo:     s.Logo.Background(bg),
		Selected: s.Selected.Background(bg),

		// Preserve internal fields
		statusColors: s.statusColors,
		background:   s.background,
	}
}

// Theme definitions

var themes = map[string]Theme{
	"Dracula": draculaTheme(),
	"Slate":   slateTheme(),
}

var themeOrder = []string{"Dracula", "Slate"}

// GetTheme returns a theme by name.
func GetTheme(name string) Theme {
	if t, ok := themes[name]; ok {
		return t
	}
	return draculaTheme()
}

// NextTheme returns the next theme name in the cycle.
func NextTheme(current string) string {
	for i, name := range themeOrder {
		if name == current {
			return themeOrder[(i+1)%len(themeOrder)]
		}
	}
	return themeOrder[0]
}

// ThemeNames returns available theme names.
func ThemeNames() []string {
	return themeOrder
}

func draculaTheme() Theme {
	// Official Dracula palette: https://draculatheme.com/spec
	// UI hierarchy from dracula/visual-studio-code canonical implementation
	return Theme{
		Name: "Dracula",

		// Base colors
		Background: "#191A21", // BGDarker (outermost, darkest)
		Surface:    "#282A36", // Background (main content panels)
		SurfaceAlt: "#21222C", // BGDark (secondary surfaces)
		FocusBg:    "#343746", // BGLight (focus/active states)

		// Table colors (matching tview Table palette exactly)
		SelectionBg:   "#44475A", // Selection
		SelectionText: "#F8F8F2", // Foreground

		// Border colors (matching tview Border palette)
		Border:      "#44475A", // Selection
		BorderMuted: "#21222C", // BGDark
		BorderFocus: "#BD93F9", // Purple

		// Text colors
		Text:    "#F8F8F2", // Foreground
		Muted:   "#6272A4", // Comment
		Faint:   "#44475A", // Selection (dimmest readable)
		Accent:  "#BD93F9", // Purple
		Success: "#50FA7B", // Green
		Warning: "#FFB86C", // Orange
		Danger:  "#FF5555", // Red
		Info:    "#8BE9FD", // Cyan

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
	}
}

func slateTheme() Theme {
	// Tailwind CSS Slate/Sky palette: https://tailwindcss.com/docs/colors
	// UI hierarchy from shadcn/ui theming
	return Theme{
		Name: "Slate",

		// Base colors
		Background: "#020617", // slate-950
		Surface:    "#0f172a", // slate-900
		SurfaceAlt: "#1e293b", // slate-800
		FocusBg:    "#283548", // between slate-800 and slate-700

		// Table colors (matching tview Table palette exactly)
		SelectionBg:   "#0284c7", // sky-600
		SelectionText: "#f8fafc", // slate-50

		// Border colors (matching tview Border palette)
		Border:      "#334155", // slate-700
		BorderMuted: "#1e293b", // slate-800
		BorderFocus: "#38bdf8", // sky-400

		// Text colors
		Text:    "#f1f5f9", // slate-100
		Muted:   "#94a3b8", // slate-400
		Faint:   "#64748b", // slate-500
		Accent:  "#38bdf8", // sky-400
		Success: "#22c55e", // green-500
		Warning: "#f59e0b", // amber-500
		Danger:  "#ef4444", // red-500
		Info:    "#06b6d4", // cyan-500

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
	}
}
