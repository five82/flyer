package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines all keyboard bindings for the application.
type keyMap struct {
	// Global
	Quit       key.Binding
	Help       key.Binding
	CycleTheme key.Binding
	Tab        key.Binding
	ShiftTab   key.Binding
	Escape     key.Binding

	// View switching
	ViewQueue      key.Binding
	ViewDaemonLogs key.Binding
	ViewItemLogs   key.Binding
	ViewProblems   key.Binding

	// Queue actions
	CycleFilter    key.Binding
	ToggleEpisodes key.Binding
	TogglePaths    key.Binding

	// Navigation
	Up           key.Binding
	Down         key.Binding
	Top          key.Binding
	Bottom       key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding

	// Logs actions
	ToggleFollow    key.Binding
	ToggleLogSource key.Binding
	Search          key.Binding
	NextMatch       key.Binding
	PrevMatch       key.Binding
	LogFilters      key.Binding

	// Search/input
	Confirm key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() keyMap {
	return keyMap{
		// Global
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "e"),
			key.WithHelp("e", "Quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("h", "?"),
			key.WithHelp("h/?", "Toggle help"),
		),
		CycleTheme: key.NewBinding(
			key.WithKeys("T"),
			key.WithHelp("T", "Cycle theme"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "Cycle views"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "Cycle views (reverse)"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "Return to queue"),
		),

		// View switching
		ViewQueue: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "Queue view"),
		),
		ViewDaemonLogs: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "Daemon logs"),
		),
		ViewItemLogs: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "Item logs"),
		),
		ViewProblems: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "Problems view"),
		),

		// Queue actions
		CycleFilter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "Cycle filter"),
		),
		ToggleEpisodes: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "Toggle episodes"),
		),
		TogglePaths: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "Toggle paths"),
		),

		// Navigation
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/up", "Move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/down", "Move down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "Go to top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "Go to bottom"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "Page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "Page down"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "Half page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "Half page down"),
		),

		// Logs actions
		ToggleFollow: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("Space", "Toggle follow mode"),
		),
		ToggleLogSource: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "Toggle item/daemon logs"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "Search logs"),
		),
		NextMatch: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "Next match"),
		),
		PrevMatch: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "Previous match"),
		),
		LogFilters: key.NewBinding(
			key.WithKeys("F"),
			key.WithHelp("F", "Log filters"),
		),

		// Search/input
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "Confirm"),
		),
	}
}

// ShortHelp returns key bindings for the short help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns key bindings for the full help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation
		{k.Tab, k.ViewQueue, k.ViewDaemonLogs, k.ViewItemLogs, k.ViewProblems},
		{k.Up, k.Down, k.Top, k.Bottom},
		{k.HalfPageDown, k.HalfPageUp},
		// Queue
		{k.CycleFilter, k.ToggleEpisodes, k.TogglePaths},
		// Logs
		{k.ToggleFollow, k.Search, k.NextMatch, k.PrevMatch, k.LogFilters},
		// General
		{k.CycleTheme, k.Help, k.Quit},
	}
}
