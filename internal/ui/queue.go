package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

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
		// Then by status rank
		pi := statusRank(items[i].Status)
		pj := statusRank(items[j].Status)
		if pi != pj {
			return pi < pj
		}
		// Then by most recent timestamp
		ti := mostRecentTimestamp(items[i])
		tj := mostRecentTimestamp(items[j])
		if ti.IsZero() && !tj.IsZero() {
			return false
		}
		if tj.IsZero() && !ti.IsZero() {
			return true
		}
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		// Finally by ID (higher IDs are newer)
		return items[i].ID > items[j].ID
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
	tablePane := m.renderTitledBox(tableTitle, tableContent, tableWidth, contentHeight, tableFocused)

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
	detailPane := m.renderTitledBox(detailTitle, detailContent, detailWidth, contentHeight, detailFocused)

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
// Format: "#ID Title · Status Progress%"
// When selected is true, uses SelectionText color for all text to ensure contrast.
func (m Model) formatQueueRowContent(item spindle.QueueItem, width int, bgColor string, selected bool) string {
	bg := NewBgStyle(bgColor)

	title := composeTitle(item)
	status := titleCase(effectiveQueueStage(item))

	// Build status parts like tview
	statusParts := []string{status}
	if isProcessingStatus(item.Status) && item.Progress.Percent > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%.0f%%", min(item.Progress.Percent, 100)))
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		statusParts = append(statusParts, "!")
	}
	if item.NeedsReview {
		statusParts = append(statusParts, "R")
	}
	statusStr := strings.Join(statusParts, " ")

	// Calculate available title width
	idStr := fmt.Sprintf("#%d", item.ID)
	separatorLen := 3 // " · "
	titleWidth := max(width-len(idStr)-len(statusStr)-separatorLen-2, 10)

	// For selected rows, use SelectionText for all parts to ensure contrast
	// For non-selected rows, use themed colors
	var idStyle, titleStyle, sepStyle, statusStyle lipgloss.Style
	if selected {
		selText := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SelectionText))
		idStyle = selText
		titleStyle = selText
		sepStyle = selText
		statusStyle = selText
	} else {
		styles := m.theme.Styles()
		idStyle = styles.MutedText
		titleStyle = styles.Text
		sepStyle = styles.FaintText
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.colorForStatus(item.Status)))
	}

	idPart := bg.Render(idStr, idStyle)
	titlePart := bg.Render(truncate(title, titleWidth), titleStyle)
	sepPart := bg.Render(" · ", sepStyle)
	statusPart := bg.Render(statusStr, statusStyle)

	return idPart + bg.Space() + titlePart + sepPart + statusPart
}

// colorForStatus returns the theme color for a given status.
func (m Model) colorForStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if color, ok := m.theme.StatusColors[status]; ok {
		return color
	}
	return m.theme.Text
}

// renderTitledBox renders content in a box with the title embedded in the top border.
// Matches tview's frame style: ┌─── Title ───┐
// When focused is true, uses BorderFocus color and FocusBg background (matching tview's focus behavior).
func (m Model) renderTitledBox(title, content string, width, height int, focused bool) string {
	// Use focus colors when focused (matching tview applyFocusStyles)
	var borderColorStr, bgColorStr string
	if focused {
		borderColorStr = m.theme.BorderFocus
		bgColorStr = m.theme.FocusBg
	} else {
		borderColorStr = m.theme.Border
		bgColorStr = m.theme.SurfaceAlt
	}
	bg := NewBgStyle(bgColorStr)
	borderColor := lipgloss.Color(borderColorStr)
	bgColor := lipgloss.Color(bgColorStr)
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Text))

	// Build the top border with embedded title
	innerWidth := width - 2 // Account for left and right border chars
	titleLen := len(title)
	leftPad := (innerWidth - titleLen - 2) / 2 // -2 for spaces around title
	rightPad := innerWidth - titleLen - 2 - leftPad

	topBorder := bg.Render("┌", borderStyle) +
		bg.Render(strings.Repeat("─", leftPad), borderStyle) +
		bg.Render(" "+title+" ", titleStyle) +
		bg.Render(strings.Repeat("─", rightPad), borderStyle) +
		bg.Render("┐", borderStyle)

	// Build the bottom border
	bottomBorder := bg.Render("└", borderStyle) +
		bg.Render(strings.Repeat("─", innerWidth), borderStyle) +
		bg.Render("┘", borderStyle)

	// Style for side borders and content background
	contentStyle := lipgloss.NewStyle().Width(innerWidth).Background(bgColor)

	// Split content into lines and pad to fill the box
	contentLines := strings.Split(content, "\n")
	boxHeight := height - 2 // -2 for top and bottom borders

	// Pad or truncate content lines
	var paddedLines []string
	for i := 0; i < boxHeight; i++ {
		var line string
		if i < len(contentLines) {
			line = contentLines[i]
		}
		// Ensure line is exactly innerWidth chars with background color
		paddedLines = append(paddedLines,
			bg.Render("│", borderStyle)+
				contentStyle.Render(line)+
				bg.Render("│", borderStyle))
	}

	// Join all parts
	return topBorder + "\n" + strings.Join(paddedLines, "\n") + "\n" + bottomBorder
}

// statusRank returns the priority rank for a status (lower = higher priority).
func statusRank(status string) int {
	ranks := map[string]int{
		"failed":              0,
		"review":              1,
		"encoding":            2,
		"subtitling":          3,
		"ripping":             4,
		"identifying":         5,
		"episode_identifying": 6,
		"organizing":          7,
		"subtitled":           8,
		"encoded":             9,
		"ripped":              10,
		"identified":          11,
		"episode_identified":  12,
		"pending":             13,
		"completed":           14,
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

// mostRecentTimestamp returns the most recent timestamp for an item.
func mostRecentTimestamp(item spindle.QueueItem) time.Time {
	var latest time.Time

	if t, err := time.Parse(time.RFC3339, item.UpdatedAt); err == nil {
		if t.After(latest) {
			latest = t
		}
	}
	if t, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
		if t.After(latest) {
			latest = t
		}
	}

	return latest
}
