package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

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

// getSelectedItem returns the currently selected queue item.
func (m *Model) getSelectedItem() *spindle.QueueItem {
	items := m.getSortedItems()
	if m.selectedRow < 0 || m.selectedRow >= len(items) {
		return nil
	}
	return &items[m.selectedRow]
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

// queueColumns holds the computed fixed column widths for the queue table.
type queueColumns struct {
	strip int
	id    int
	stage int
	pct   int
	ago   int
	title int
}

// computeQueueColumns derives column widths from the item set and terminal
// width. The title column absorbs the slack.
func computeQueueColumns(items []spindle.QueueItem, width int) queueColumns {
	cols := queueColumns{strip: 1, id: 2, stage: 12, pct: 4, ago: 8}
	for _, item := range items {
		if n := len(item.Tasks); n > cols.strip {
			cols.strip = n
		}
		idLen := len(fmt.Sprintf("#%d", item.ID)) + 1 // room for review "?"
		if idLen > cols.id {
			cols.id = idLen
		}
	}

	// strip + id + stage + pct + ago + 5 separators (2 spaces each)
	fixed := cols.strip + cols.id + cols.stage + cols.pct + cols.ago + 10
	cols.title = max(width-fixed, 10)
	return cols
}

// renderQueue renders the dashboard queue table (full width).
// Chrome above: header, NOW band, command bar; plus the title rule here.
func (m Model) renderQueue() string {
	styles := m.theme.Styles()
	visibleRows := max(m.height-4, 1)

	var b strings.Builder
	b.WriteString(renderRule(m.getQueueTitle(), m.width, styles))
	b.WriteString("\n")

	items := m.getSortedItems()
	if len(items) == 0 {
		msg := "No items in queue"
		if m.filterMode != FilterAll {
			msg = "No items match filter: " + m.filterLabel()
		}
		b.WriteString(styles.MutedText.Render(msg))
		return b.String()
	}

	// Keep the selection visible within the scroll window. The stored
	// offset is maintained on key handling; re-derive here defensively so a
	// resize between keypresses cannot hide the selection.
	scroll := clampQueueScroll(m.queueScroll, m.selectedRow, visibleRows, len(items))

	cols := computeQueueColumns(items, m.width)
	end := min(scroll+visibleRows, len(items))
	for i := scroll; i < end; i++ {
		b.WriteString(m.renderQueueRow(items[i], cols, i == m.selectedRow, styles))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// clampQueueScroll adjusts a scroll offset so the selection stays visible
// and the window stays within bounds.
func clampQueueScroll(scroll, selected, visible, total int) int {
	if selected < scroll {
		scroll = selected
	}
	if selected >= scroll+visible {
		scroll = selected - visible + 1
	}
	return max(min(scroll, total-visible), 0)
}

// ensureQueueVisible updates the stored scroll offset after selection moves.
func (m *Model) ensureQueueVisible() {
	visible := max(m.height-4, 1)
	m.queueScroll = clampQueueScroll(m.queueScroll, m.selectedRow, visible, len(m.getSortedItems()))
}

// renderQueueRow renders one queue table row:
// strip  id  title  stage  pct  ago
// The selected row renders as one selection-colored bar (no per-cell colors,
// guaranteeing contrast); other rows use per-cell styling.
func (m Model) renderQueueRow(item spindle.QueueItem, cols queueColumns, selected bool, styles Styles) string {
	idStr := fmt.Sprintf("#%d", item.ID)
	if item.NeedsReview {
		idStr += "?"
	}
	title := truncate(composeTitle(item), cols.title)
	stage, stageStyle := queueStageCell(item, styles)
	pct := queuePercentCell(item)
	ago := ""
	if updated := parseTimestamp(item.UpdatedAt); !updated.IsZero() {
		ago = humanizeDuration(time.Since(updated))
	}

	pad := func(s string, w int) string {
		if n := w - lipgloss.Width(s); n > 0 {
			return s + strings.Repeat(" ", n)
		}
		return s
	}

	if selected {
		line := fmt.Sprintf("%s  %s  %s  %s  %s  %s",
			pad(plainTaskStrip(item), cols.strip),
			pad(idStr, cols.id),
			pad(title, cols.title),
			pad(stage, cols.stage),
			pad(pct, cols.pct),
			ago)
		if n := m.width - lipgloss.Width(line); n > 0 {
			line += strings.Repeat(" ", n)
		}
		return styles.Selected.Render(line)
	}

	idStyle := styles.MutedText
	if item.NeedsReview {
		idStyle = styles.WarningText
	}

	parts := []string{
		pad(m.renderTaskStrip(item, styles), cols.strip),
		idStyle.Render(pad(idStr, cols.id)),
		styles.Text.Render(pad(title, cols.title)),
		stageStyle.Render(pad(stage, cols.stage)),
		styles.AccentText.Render(pad(pct, cols.pct)),
		styles.FaintText.Render(ago),
	}
	return strings.Join(parts, "  ")
}

// queueStageCell returns the stage column text and style for an item.
func queueStageCell(item spindle.QueueItem, styles Styles) (string, lipgloss.Style) {
	if item.NeedsReview {
		return "REVIEW", styles.WarningText
	}
	if strings.EqualFold(item.Stage, "failed") {
		return "FAILED", styles.DangerText
	}
	info := stageDisplay(itemDisplayStage(item))
	label := info.label
	style := roleStyle(info.role, styles)
	if item.IsTerminal() {
		label = info.doneLabel
		style = styles.MutedText
	} else if len(item.RunningTasks()) == 0 {
		label = "waiting"
		style = styles.FaintText
	}
	return strings.ToLower(label), style
}

// queuePercentCell returns the progress column text for an item: the primary
// running task's percent, or blank.
func queuePercentCell(item spindle.QueueItem) string {
	for _, t := range item.Tasks {
		if t.IsRunning() && t.Progress.Percent > 0 {
			return fmt.Sprintf("%3.0f%%", clampPercent(t.Progress.Percent))
		}
	}
	return ""
}

// renderTaskStrip renders one glyph per task, colored by task state (with
// the running glyph in its stage's role color). Terminal or task-less items
// collapse to a single summary glyph.
func (m Model) renderTaskStrip(item spindle.QueueItem, styles Styles) string {
	if len(item.Tasks) == 0 {
		glyph, style := "○", styles.FaintText
		switch {
		case strings.EqualFold(item.Stage, "completed"):
			glyph, style = "✓", styles.SuccessText
		case strings.EqualFold(item.Stage, "failed"):
			glyph, style = "✗", styles.DangerText
		}
		return style.Render(glyph)
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
		b.WriteString(style.Render(taskStateGlyph(t.State)))
	}
	return b.String()
}

// plainTaskStrip renders the task strip without styling (for selected rows).
func plainTaskStrip(item spindle.QueueItem) string {
	if len(item.Tasks) == 0 {
		switch {
		case strings.EqualFold(item.Stage, "completed"):
			return "✓"
		case strings.EqualFold(item.Stage, "failed"):
			return "✗"
		default:
			return "○"
		}
	}
	var b strings.Builder
	for _, t := range item.Tasks {
		b.WriteString(taskStateGlyph(t.State))
	}
	return b.String()
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

// getQueueTitle returns the queue rule title with optional filter indicator.
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
