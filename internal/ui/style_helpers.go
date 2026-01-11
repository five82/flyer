package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// BgStyle provides helpers for rendering text with consistent background colors.
// This solves lipgloss's limitation where ANSI reset codes between styled segments
// cause gaps in background color. See: https://github.com/charmbracelet/lipgloss/discussions/78
type BgStyle struct {
	bg    lipgloss.Color
	space string // cached styled space
}

// NewBgStyle creates a new background style helper for the given color.
func NewBgStyle(bgColor string) BgStyle {
	bg := lipgloss.Color(bgColor)
	return BgStyle{
		bg:    bg,
		space: lipgloss.NewStyle().Background(bg).Render(" "),
	}
}

// Render renders text with a style, ensuring ALL characters including spaces
// have the background color applied.
func (b BgStyle) Render(text string, style lipgloss.Style) string {
	if text == "" {
		return ""
	}

	// If no spaces, simple render with background
	if !strings.Contains(text, " ") {
		return style.Background(b.bg).Render(text)
	}

	// Split on spaces, style each word, rejoin with styled spaces
	wordStyle := style.Background(b.bg)
	words := strings.Split(text, " ")
	result := make([]string, 0, len(words))
	for _, w := range words {
		if w != "" {
			result = append(result, wordStyle.Render(w))
		} else {
			// Preserve multiple consecutive spaces
			result = append(result, "")
		}
	}
	return strings.Join(result, b.space)
}

// Space returns a single styled space.
func (b BgStyle) Space() string {
	return b.space
}

// Spaces returns n styled spaces.
func (b BgStyle) Spaces(n int) string {
	return lipgloss.NewStyle().Background(b.bg).Render(strings.Repeat(" ", n))
}

// Sep returns a styled separator string.
func (b BgStyle) Sep(sep string) string {
	return lipgloss.NewStyle().Background(b.bg).Render(sep)
}

// Join joins parts with a styled separator.
func (b BgStyle) Join(parts []string, sep string) string {
	return strings.Join(parts, b.Sep(sep))
}

// Color returns the background color.
func (b BgStyle) Color() lipgloss.Color {
	return b.bg
}

// FillLine pads rendered content to fill the specified width with the background color.
// Use this to ensure lines fill the full viewport width.
func (b BgStyle) FillLine(content string, width int) string {
	return lipgloss.NewStyle().Background(b.bg).Width(width).Render(content)
}
