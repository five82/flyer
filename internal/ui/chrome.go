package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// renderRule renders a full-width horizontal rule with an embedded title:
// ── Title ───────────────
// The title renders bold; the dashes use the theme border color.
func renderRule(title string, width int, styles Styles) string {
	line := styles.RuleText.Render("── ")
	if title != "" {
		line += styles.Text.Bold(true).Render(title) + styles.RuleText.Render(" ")
	}
	used := lipgloss.Width(line)
	if width > used {
		line += styles.RuleText.Render(strings.Repeat("─", width-used))
	}
	return line
}

// chip renders a status badge: theme background color text on a colored fill.
func chip(label, colorHex string, theme Theme) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Background)).
		Background(lipgloss.Color(colorHex)).
		Padding(0, 1).
		Render(label)
}

// wrapText word-wraps s to the given width and returns the resulting lines.
// Words longer than the width are hard-split. A non-positive width returns
// the input as a single line.
func wrapText(s string, width int) []string {
	s = strings.TrimSpace(s)
	if width <= 0 || lipgloss.Width(s) <= width {
		return []string{s}
	}

	var lines []string
	var line string
	for _, word := range strings.Fields(s) {
		for lipgloss.Width(word) > width {
			// Hard-split an overlong word.
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			runes := []rune(word)
			lines = append(lines, string(runes[:width]))
			word = string(runes[width:])
		}
		switch {
		case line == "":
			line = word
		case lipgloss.Width(line)+1+lipgloss.Width(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
