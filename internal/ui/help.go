package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpModal displays keyboard shortcuts.
type HelpModal struct {
	keys keyMap
}

// NewHelpModal creates a new help modal.
func NewHelpModal(keys keyMap) *HelpModal {
	return &HelpModal{keys: keys}
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

	// Build help content from keyMap
	var b strings.Builder

	// Title
	title := styles.Text.Bold(true).Render("Keyboard Shortcuts")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(styles.FaintText.Render(strings.Repeat("─", 30)))
	b.WriteString("\n\n")

	sections := h.keys.HelpSections()
	for i, section := range sections {
		// Section title
		b.WriteString(styles.AccentText.Bold(true).Render(section.Title))
		b.WriteString("\n")

		for _, binding := range section.Bindings {
			help := binding.Help()
			// Key
			keyStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.Warning)).
				Width(12)
			b.WriteString(keyStyle.Render(help.Key))
			// Description
			b.WriteString(styles.Text.Render(help.Desc))
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
