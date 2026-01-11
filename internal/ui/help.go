package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpModal displays keyboard shortcuts.
type HelpModal struct{}

// NewHelpModal creates a new help modal.
func NewHelpModal() *HelpModal {
	return &HelpModal{}
}

// Update handles input for the help modal. Any key closes it.
func (h *HelpModal) Update(msg tea.Msg, keys keyMap) (Modal, tea.Cmd, bool) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return h, nil, true // Any key closes help
	}
	return h, nil, false
}

// View renders the help modal.
func (h *HelpModal) View(theme Theme, width, height int) string {
	styles := theme.Styles()

	// Help content
	sections := []helpSection{
		{
			title: "Navigation",
			items: []helpItem{
				{"tab", "Cycle views"},
				{"q/l/i/p", "Queue/Daemon/Item/Problems"},
				{"esc", "Return to queue"},
				{"j/k", "Move up/down"},
				{"g/G", "Go to top/bottom"},
				{"ctrl+d/u", "Half page down/up"},
			},
		},
		{
			title: "Queue",
			items: []helpItem{
				{"f", "Cycle filter"},
				{"t", "Toggle episodes"},
				{"P", "Toggle paths"},
			},
		},
		{
			title: "Logs",
			items: []helpItem{
				{"Space", "Toggle follow mode"},
				{"i", "Toggle item/daemon logs"},
				{"/", "Search logs"},
				{"n/N", "Next/prev match"},
				{"F", "Log filters"},
			},
		},
		{
			title: "General",
			items: []helpItem{
				{"T", "Cycle theme"},
				{"h/?", "Toggle help"},
				{"e/ctrl+c", "Quit"},
			},
		},
	}

	// Build help content
	var b strings.Builder

	// Title
	title := styles.Text.Bold(true).Render("Keyboard Shortcuts")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(styles.FaintText.Render(strings.Repeat("─", 30)))
	b.WriteString("\n\n")

	for i, section := range sections {
		// Section title
		b.WriteString(styles.AccentText.Bold(true).Render(section.title))
		b.WriteString("\n")

		for _, item := range section.items {
			// Key
			keyStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.Warning)).
				Width(12)
			b.WriteString(keyStyle.Render(item.key))
			// Description
			b.WriteString(styles.Text.Render(item.desc))
			b.WriteString("\n")
		}

		if i < len(sections)-1 {
			b.WriteString("\n")
		}
	}

	// Build the modal
	content := b.String()

	// Calculate modal dimensions
	modalWidth := 40

	// Modal style
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Accent)).
		Padding(1, 2).
		Width(modalWidth)

	// Center the modal
	modalContent := modal.Render(content)

	// Create overlay
	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		modalContent,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color(theme.Background)),
	)
}

type helpSection struct {
	title string
	items []helpItem
}

type helpItem struct {
	key  string
	desc string
}
