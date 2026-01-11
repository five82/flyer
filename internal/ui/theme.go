package ui

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
		muted:        t.Muted,
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
	muted        string
}

// StatusStyle returns a style for the given status.
func (s Styles) StatusStyle(status string) lipgloss.Style {
	color := s.statusColors[status]
	if color == "" {
		color = s.muted // Fallback to theme's muted color
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
	"Nightfox": nightfoxTheme(),
	"Kanagawa": kanagawaTheme(),
	"Slate":    slateTheme(),
}

var themeOrder = []string{"Nightfox", "Kanagawa", "Slate"}

// GetTheme returns a theme by name.
func GetTheme(name string) Theme {
	if t, ok := themes[name]; ok {
		return t
	}
	return nightfoxTheme()
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

func nightfoxTheme() Theme {
	// Nightfox palette: https://github.com/EdenEast/nightfox.nvim
	return Theme{
		Name: "Nightfox",

		// Base colors
		Background: "#131a24", // bg0
		Surface:    "#192330", // bg1
		SurfaceAlt: "#212e3f", // bg2
		FocusBg:    "#29394f", // bg3

		// Table colors
		SelectionBg:   "#2b3b51", // sel0
		SelectionText: "#cdcecf", // fg1

		// Border colors
		Border:      "#39506d", // bg4
		BorderMuted: "#212e3f", // bg2
		BorderFocus: "#719cd6", // blue

		// Text colors
		Text:    "#cdcecf", // fg1 (cool gray)
		Muted:   "#738091", // comment (3.3:1 contrast)
		Faint:   "#71839b", // fg3 (3.1:1 contrast)
		Accent:  "#719cd6", // blue
		Success: "#81b29a", // green
		Warning: "#dbc074", // yellow
		Danger:  "#c94f6d", // red
		Info:    "#63cdcf", // cyan

		StatusColors: map[string]string{
			"pending":             "#738091", // comment
			"identifying":         "#63cdcf", // cyan
			"identified":          "#71839b", // fg3
			"ripping":             "#719cd6", // blue
			"ripped":              "#71839b", // fg3
			"episode_identifying": "#63cdcf", // cyan
			"episode_identified":  "#71839b", // fg3
			"encoding":            "#9d79d6", // magenta
			"encoded":             "#81b29a", // green
			"subtitling":          "#63cdcf", // cyan
			"subtitled":           "#81b29a", // green
			"organizing":          "#f4a261", // orange
			"completed":           "#81b29a", // green
			"failed":              "#c94f6d", // red
			"review":              "#dbc074", // yellow
		},
	}
}

func kanagawaTheme() Theme {
	// Kanagawa palette: https://github.com/rebelot/kanagawa.nvim
	return Theme{
		Name: "Kanagawa",

		// Base colors
		Background: "#16161D", // sumiInk0
		Surface:    "#1F1F28", // sumiInk3
		SurfaceAlt: "#2A2A37", // sumiInk4
		FocusBg:    "#2A2A37", // sumiInk4

		// Table colors
		SelectionBg:   "#2D4F67", // waveBlue1
		SelectionText: "#DCD7BA", // fujiWhite

		// Border colors
		Border:      "#54546D", // sumiInk6
		BorderMuted: "#2A2A37", // sumiInk4
		BorderFocus: "#7E9CD8", // crystalBlue

		// Text colors
		Text:    "#DCD7BA", // fujiWhite (warm parchment)
		Muted:   "#C8C093", // oldWhite (7.6:1 contrast)
		Faint:   "#727169", // fujiGray (2.8:1 contrast)
		Accent:  "#7E9CD8", // crystalBlue
		Success: "#98BB6C", // springGreen
		Warning: "#E6C384", // carpYellow
		Danger:  "#E46876", // waveRed
		Info:    "#7FB4CA", // springBlue

		StatusColors: map[string]string{
			"pending":             "#727169", // fujiGray
			"identifying":         "#7FB4CA", // springBlue
			"identified":          "#727169", // fujiGray
			"ripping":             "#7E9CD8", // crystalBlue
			"ripped":              "#727169", // fujiGray
			"episode_identifying": "#7FB4CA", // springBlue
			"episode_identified":  "#727169", // fujiGray
			"encoding":            "#957FB8", // oniViolet
			"encoded":             "#98BB6C", // springGreen
			"subtitling":          "#7FB4CA", // springBlue
			"subtitled":           "#98BB6C", // springGreen
			"organizing":          "#E6C384", // carpYellow
			"completed":           "#98BB6C", // springGreen
			"failed":              "#E46876", // waveRed
			"review":              "#E6C384", // carpYellow
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
			"ripping":             "#0ea5e9", // sky-500 (active)
			"ripped":              "#0369a1", // sky-700 (completed)
			"episode_identifying": "#7dd3fc", // sky-300
			"episode_identified":  "#0ea5e9", // sky-500
			"encoding":            "#06b6d4", // cyan-500 (active)
			"encoded":             "#22c55e", // green-500 (success)
			"subtitling":          "#22d3ee", // cyan-400 (active)
			"subtitled":           "#14b8a6", // teal-500 (success)
			"organizing":          "#f59e0b", // amber-500
			"completed":           "#16a34a", // green-600 (success)
			"failed":              "#dc2626", // red-600 (error)
			"review":              "#f59e0b", // amber-500 (attention)
		},
	}
}
