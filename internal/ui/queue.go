package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

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
				m.selectedRow = i
				return
			}
		}
	}

	// Item not found - clamp to valid range
	if m.selectedRow >= itemCount {
		m.selectedRow = itemCount - 1
	}
}

// getSortedItems returns queue items filtered and sorted by priority.
func (m *Model) getSortedItems() []spindle.QueueItem {
	items := make([]spindle.QueueItem, 0, len(m.snapshot.Queue))

	// Apply filter
	for _, item := range m.snapshot.Queue {
		switch m.filterMode {
		case FilterFailed:
			if !strings.EqualFold(item.Stage, "failed") {
				continue
			}
		case FilterReview:
			if !item.NeedsReview {
				continue
			}
		case FilterProcessing:
			if !isProcessingItem(item) {
				continue
			}
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		// Review first, then failed, then live work, then by ID ascending
		// (spindle's processing order).
		pi, pj := itemSortRank(items[i]), itemSortRank(items[j])
		if pi != pj {
			return pi < pj
		}
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

	// Calculate pane dimensions (responsive)
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
		detailContent = m.detailViewport.View()
	} else {
		detailContent = lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Muted)).
			Background(lipgloss.Color(detailBg)).
			Render("Select an item")
	}
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
// Format: "[task strip] #ID Title" -- the strip is one glyph per scheduler
// task in pipeline order, so concurrent work is visible in the list itself.
// When selected is true, uses SelectionText color for all text to ensure contrast.
func (m Model) formatQueueRowContent(item spindle.QueueItem, width int, bgColor string, selected bool) string {
	bg := NewBgStyle(bgColor)
	styles := m.theme.Styles()

	title := composeTitle(item)
	idStr := fmt.Sprintf("#%d", item.ID)
	if item.NeedsReview {
		idStr += "?"
	}

	var selText lipgloss.Style
	if selected {
		selText = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SelectionText))
	}

	strip, stripWidth := m.renderTaskStrip(item, selected, selText, styles, bg)

	// Calculate available title width: total - strip - space - id - space
	fixedLen := stripWidth + 1 + len(idStr) + 1
	titleWidth := max(width-fixedLen, 10)

	idStyle, titleStyle := styles.MutedText, styles.Text
	if selected {
		idStyle, titleStyle = selText, selText
	} else if item.NeedsReview {
		idStyle = styles.WarningText
	}

	idPart := bg.Render(idStr, idStyle)
	titlePart := bg.Render(truncate(title, titleWidth), titleStyle)

	return strip + bg.Space() + idPart + bg.Space() + titlePart
}

// renderTaskStrip renders one glyph per task, colored by task state (with
// the running glyph in its stage's role color). Terminal or task-less items
// collapse to a single summary glyph. Returns the strip and its cell width.
func (m Model) renderTaskStrip(item spindle.QueueItem, selected bool, selText lipgloss.Style, styles Styles, bg BgStyle) (string, int) {
	styleFor := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return selText
		}
		return s
	}

	if len(item.Tasks) == 0 {
		glyph, style := "○", styles.FaintText
		switch {
		case strings.EqualFold(item.Stage, "completed"):
			glyph, style = "✓", styles.SuccessText
		case strings.EqualFold(item.Stage, "failed"):
			glyph, style = "✗", styles.DangerText
		}
		return bg.Render(glyph, styleFor(style)), 1
	}

	var b strings.Builder
	for _, t := range item.Tasks {
		style := styles.FaintText
		switch t.State {
		case "done":
			style = styles.SuccessText
		case "running":
			style = roleStyle(stageDisplay(t.Type).role, styles)
		case "failed":
			style = styles.DangerText
		}
		b.WriteString(bg.Render(taskStateGlyph(t.State), styleFor(style)))
	}
	return b.String(), len(item.Tasks)
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

// composeTitle builds the display title for an item, preferring the
// server-computed one.
func composeTitle(item spindle.QueueItem) string {
	if item.DisplayTitle != "" {
		return item.DisplayTitle
	}
	if item.DiscTitle != "" {
		return item.DiscTitle
	}
	return fmt.Sprintf("Item #%d", item.ID)
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
