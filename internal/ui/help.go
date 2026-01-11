package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHelp renders the help overlay.
func (m Model) renderHelp() string {
	styles := m.theme.Styles()

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
				Foreground(lipgloss.Color(m.theme.Warning)).
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
		BorderForeground(lipgloss.Color(m.theme.Accent)).
		Padding(1, 2).
		Width(modalWidth)

	// Center the modal
	modalContent := modal.Render(content)

	// Create overlay
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modalContent,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color(m.theme.Background)),
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
