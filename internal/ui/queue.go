package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/five82/flyer/internal/spindle"
)

// updateQueueTable updates selection bounds when queue changes.
// Preserves selection by item ID when possible.
func (m *Model) updateQueueTable() {
	// Get the currently selected item's ID before updating
	var selectedID int64
	if item := m.getSelectedItem(); item != nil {
		selectedID = item.ID
	}

	items := m.getSortedItems()
	itemCount := len(items)

	if itemCount == 0 {
		m.selectedRow = 0
		return
	}

	// Try to find the previously selected item by ID
	if selectedID > 0 {
		for i, item := range items {
			if item.ID == selectedID {
				if m.selectedRow != i {
					m.detailScroll = 0 // Reset scroll on selection change
				}
				m.selectedRow = i
				return
			}
		}
	}

	// Item not found - clamp to valid range and reset scroll
	if m.selectedRow >= itemCount {
		m.selectedRow = itemCount - 1
		m.detailScroll = 0
	}
}

// getSortedItems returns queue items filtered and sorted by priority.
func (m *Model) getSortedItems() []spindle.QueueItem {
	items := make([]spindle.QueueItem, 0, len(m.snapshot.Queue))

	// Apply filter
	for _, item := range m.snapshot.Queue {
		switch m.filterMode {
		case FilterFailed:
			if !strings.EqualFold(item.Status, "failed") {
				continue
			}
		case FilterReview:
			if !item.NeedsReview {
				continue
			}
		case FilterProcessing:
			if !isProcessingStatus(item.Status) {
				continue
			}
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		// Needs review items first
		if items[i].NeedsReview != items[j].NeedsReview {
			return items[i].NeedsReview
		}
		// Then by status rank (active items bubble up)
		pi := statusRank(items[i].Status)
		pj := statusRank(items[j].Status)
		if pi != pj {
			return pi < pj
		}
		// Then by ID ascending (matches Spindle's processing order)
		return items[i].ID < items[j].ID
	})

	return items
}

// renderQueue renders the queue view with split layout (table + detail).
func (m Model) renderQueue() string {
	styles := m.theme.Styles()
	contentHeight := m.height - 2 // Account for header + cmdbar

	if len(m.snapshot.Queue) == 0 {
		emptyMsg := styles.MutedText.Render("No items in queue")
		return lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, emptyMsg)
	}

	// Check if filter results in empty list
	filteredItems := m.getSortedItems()
	if len(filteredItems) == 0 && m.filterMode != FilterAll {
		msg := m.emptyFilterMessage()
		emptyMsg := styles.MutedText.Render(msg) + "\n" +
			styles.FaintText.Render("Press f to change filter")
		return lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, emptyMsg)
	}

	// Calculate pane dimensions (responsive like tview)
	// Extra wide (>= 160): 30% table, 70% detail
	// Default: 40% table, 60% detail
	var tableWidth, detailWidth int
	if m.width >= 160 {
		tableWidth = m.width * 30 / 100
	} else {
		tableWidth = m.width * 40 / 100
	}
	detailWidth = m.width - tableWidth

	// Get selected item
	item := m.getSelectedItem()

	// === Table Pane ===
	tableTitle := m.getQueueTitle()
	tableFocused := m.focusedPane == 0
	tableBg := m.theme.SurfaceAlt
	if tableFocused {
		tableBg = m.theme.FocusBg
	}
	tableContent := m.renderQueueTable(tableWidth-2, tableBg) // -2 for borders
	tablePane := m.renderBox(tableTitle, tableContent, tableWidth, contentHeight, tableFocused)

	// === Detail Pane ===
	detailTitle := "Details"
	detailFocused := m.focusedPane == 1
	detailBg := m.theme.SurfaceAlt
	if detailFocused {
		detailBg = m.theme.FocusBg
	}
	var detailContent string
	if item != nil {
		detailContent = m.renderDetailContent(*item, detailWidth-4, detailBg)
	} else {
		detailContent = lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Muted)).
			Background(lipgloss.Color(detailBg)).
			Render("Select an item")
	}

	// Apply scroll offset to detail content
	detailContent = m.applyDetailScroll(detailContent, contentHeight-2)

	detailPane := m.renderBox(detailTitle, detailContent, detailWidth, contentHeight, detailFocused)

	// Join side-by-side
	return lipgloss.JoinHorizontal(lipgloss.Top, tablePane, detailPane)
}

// renderQueueTable renders the queue as styled rows.
func (m Model) renderQueueTable(width int, bgColor string) string {
	items := m.getSortedItems()
	if len(items) == 0 {
		return ""
	}

	// Build rows directly (no table component overhead)
	var lines []string
	for i, item := range items {
		if i == m.selectedRow {
			// Selected row: use selection background and text color
			content := m.formatQueueRowContent(item, width, m.theme.SelectionBg, true)
			line := lipgloss.NewStyle().
				Background(lipgloss.Color(m.theme.SelectionBg)).
				Width(width).
				Render(content)
			lines = append(lines, line)
		} else {
			// Non-selected row: use pane background with themed colors
			content := m.formatQueueRowContent(item, width, bgColor, false)
			line := lipgloss.NewStyle().
				Background(lipgloss.Color(bgColor)).
				Width(width).
				Render(content)
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// formatQueueRowContent formats a queue item row with inline colors.
// Format: "Icon #ID Title         62%" for active items, "Icon #ID Title" for others.
// When selected is true, uses SelectionText color for all text to ensure contrast.
func (m Model) formatQueueRowContent(item spindle.QueueItem, width int, bgColor string, selected bool) string {
	bg := NewBgStyle(bgColor)

	title := composeTitle(item)
	icon := stageIcon(effectiveQueueStage(item))
	idStr := fmt.Sprintf("#%d", item.ID)

	// Check for inline progress
	progressStr := m.formatInlineProgress(item)
	progressLen := len(progressStr)
	if progressLen > 0 {
		progressLen += 1 // account for leading space
	}

	// Calculate available title width: total - icon - space - id - space - progress
	fixedLen := 1 + 1 + len(idStr) + 1 + progressLen // icon + space + id + space + progress
	titleWidth := max(width-fixedLen, 10)

	// For selected rows, use SelectionText for all parts to ensure contrast
	// For non-selected rows, use themed colors
	var iconStyle, idStyle, titleStyle, progressStyle lipgloss.Style
	if selected {
		selText := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SelectionText))
		iconStyle = selText
		idStyle = selText
		titleStyle = selText
		progressStyle = selText
	} else {
		styles := m.theme.Styles()
		iconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.colorForStatus(effectiveQueueStage(item))))
		idStyle = styles.MutedText
		titleStyle = styles.Text
		progressStyle = styles.AccentText
	}

	iconPart := bg.Render(icon, iconStyle)
	idPart := bg.Render(idStr, idStyle)
	titlePart := bg.Render(truncate(title, titleWidth), titleStyle)

	row := iconPart + bg.Space() + idPart + bg.Space() + titlePart

	if progressStr != "" {
		row += bg.Space() + bg.Render(progressStr, progressStyle)
	}

	return row
}

// formatInlineProgress returns a compact progress string for active items.
// Returns empty string for items without meaningful progress.
func (m Model) formatInlineProgress(item spindle.QueueItem) string {
	if !isProcessingStatus(item.Status) {
		return ""
	}

	// Use encoding progress if available (most accurate)
	if enc := item.Encoding; enc != nil && strings.EqualFold(item.Status, "encoding") {
		percent := enc.Percent
		if percent <= 0 && enc.TotalFrames > 0 && enc.CurrentFrame > 0 {
			percent = (float64(enc.CurrentFrame) / float64(enc.TotalFrames)) * 100
		}
		if percent > 0 {
			return fmt.Sprintf("%3.0f%%", percent)
		}
	}

	// Fall back to general progress
	if item.Progress.Percent > 0 && item.Progress.Percent < 100 {
		return fmt.Sprintf("%3.0f%%", item.Progress.Percent)
	}

	return ""
}

// stageIcons maps status to display icon.
var stageIcons = map[string]string{
	"pending":             "~",
	"identifying":         "*",
	"episode_identifying": "*",
	"identified":          "*",
	"episode_identified":  "*",
	"ripping":             ">",
	"ripped":              ">",
	"encoding":            "%",
	"encoded":             "%",
	"audio_analyzing":     "#",
	"audio_analyzed":      "#",
	"subtitling":          "@",
	"subtitled":           "@",
	"organizing":          "+",
	"completed":           "+",
	"failed":              "!",
	"review":              "?",
}

// stageIcon returns a display icon for the given status.
func stageIcon(status string) string {
	if icon, ok := stageIcons[strings.ToLower(strings.TrimSpace(status))]; ok {
		return icon
	}
	return "-"
}

// colorForStatus returns the theme color for a given status.
func (m Model) colorForStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if color, ok := m.theme.StatusColors[status]; ok {
		return color
	}
	return m.theme.Text
}

// renderBox renders content in a titled box with the title embedded in the top border.
// Example: ╭─── Title ─────────────────────────────╮
// When focused is true, uses BorderFocus color and FocusBg background.
func (m Model) renderBox(title, content string, width, height int, focused bool) string {
	var borderColor, bgColor string
	if focused {
		borderColor = m.theme.BorderFocus
		bgColor = m.theme.FocusBg
	} else {
		borderColor = m.theme.Border
		bgColor = m.theme.SurfaceAlt
	}

	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(borderColor)).
		Background(lipgloss.Color(bgColor))

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.theme.Text)).
		Background(lipgloss.Color(bgColor))

	contentBgStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(bgColor))

	// Build top border: ╭─── Title ─────────────────────────────╮
	titleText := " " + title + " "
	innerWidth := width - 2 // minus corners
	titleLen := lipgloss.Width(titleText)
	leftDashes := 3
	rightDashes := max(innerWidth-leftDashes-titleLen, 0)

	topLine := borderStyle.Render("╭"+strings.Repeat("─", leftDashes)) +
		titleStyle.Render(titleText) +
		borderStyle.Render(strings.Repeat("─", rightDashes)+"╮")

	// Build content lines with side borders
	// Available height for content: total height - 2 (top/bottom border)
	contentHeight := height - 2
	contentWidth := width - 2 // minus side borders

	contentLines := strings.Split(content, "\n")
	var middleLines []string

	for i := range contentHeight {
		var line string
		if i < len(contentLines) {
			line = contentLines[i]
		}

		// Pad line to fill width, accounting for ANSI sequences
		lineWidth := lipgloss.Width(line)
		if lineWidth < contentWidth {
			line += contentBgStyle.Render(strings.Repeat(" ", contentWidth-lineWidth))
		}

		middleLine := borderStyle.Render("│") + line + borderStyle.Render("│")
		middleLines = append(middleLines, middleLine)
	}

	// Build bottom border: ╰─────────────────────────────────────╯
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	return topLine + "\n" + strings.Join(middleLines, "\n") + "\n" + bottomLine
}

// statusRank returns the priority rank for a status (lower = higher priority).
func statusRank(status string) int {
	ranks := map[string]int{
		"failed":              0,
		"review":              1,
		"encoding":            2,
		"subtitling":          3,
		"ripping":             4,
		"audio_analyzing":     5,
		"identifying":         6,
		"episode_identifying": 7,
		"organizing":          8,
		"subtitled":           9,
		"encoded":             10,
		"ripped":              11,
		"audio_analyzed":      12,
		"identified":          13,
		"episode_identified":  14,
		"pending":             15,
		"completed":           16,
	}
	if r, ok := ranks[strings.ToLower(status)]; ok {
		return r
	}
	return 100
}

// isProcessingStatus returns true if the status indicates active processing.
func isProcessingStatus(status string) bool {
	_, ok := processingStatuses[strings.ToLower(strings.TrimSpace(status))]
	return ok
}

// composeTitle builds the display title for an item.
func composeTitle(item spindle.QueueItem) string {
	if item.DiscTitle != "" {
		return item.DiscTitle
	}
	if item.SourcePath != "" {
		// Extract filename from path
		parts := strings.Split(item.SourcePath, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return item.SourcePath
	}
	return fmt.Sprintf("Item #%d", item.ID)
}

// effectiveQueueStage returns the display status for an item.
func effectiveQueueStage(item spindle.QueueItem) string {
	if item.NeedsReview {
		return "review"
	}
	return item.Status
}

// titleCase converts a snake_case or lowercase string to Title Case.
func titleCase(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// emptyFilterMessage returns a contextual message when a filter matches no items.
func (m Model) emptyFilterMessage() string {
	switch m.filterMode {
	case FilterFailed:
		return "No failed items"
	case FilterReview:
		return "No items need review"
	case FilterProcessing:
		return "No active items"
	default:
		return "No items match filter"
	}
}

// applyDetailScroll applies scroll offset to the detail content string.
// It skips the first m.detailScroll lines of content, clamping to available lines.
func (m *Model) applyDetailScroll(content string, viewHeight int) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Clamp scroll to valid range
	maxScroll := max(totalLines-viewHeight, 0)
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}

	// Apply offset
	if m.detailScroll > 0 && m.detailScroll < totalLines {
		lines = lines[m.detailScroll:]
	}

	return strings.Join(lines, "\n")
}

// getQueueTitle returns the queue pane title with optional filter indicator.
func (m Model) getQueueTitle() string {
	items := m.getSortedItems()
	total := len(m.snapshot.Queue)
	visible := len(items)

	if m.filterMode == FilterAll {
		return fmt.Sprintf("Queue (%d)", total)
	}

	// Show "Queue (visible/total) - FilterName"
	return fmt.Sprintf("Queue (%d/%d) %s", visible, total, m.filterLabel())
}
