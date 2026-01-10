package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

// processingStatuses defines which statuses are considered "processing".
var processingStatuses = map[string]struct{}{
	"identifying":         {},
	"ripping":             {},
	"episode_identifying": {},
	"episode_identified":  {},
	"encoding":            {},
	"subtitling":          {},
	"subtitled":           {},
	"organizing":          {},
}

// isProcessingStatus returns true if the status is a processing status.
func isProcessingStatus(status string) bool {
	_, ok := processingStatuses[strings.ToLower(strings.TrimSpace(status))]
	return ok
}

// itemToRow converts an item index (0-based) to the table row.
func itemToRow(itemIdx int) int {
	return itemIdx
}

// rowToItem converts a table row to the item index.
func rowToItem(row int) int {
	if row < 0 {
		return -1
	}
	return row
}

func (vm *viewModel) renderTable() {
	vm.table.Clear()

	rows := append([]spindle.QueueItem(nil), vm.items...)
	switch vm.filterMode {
	case filterFailed:
		rows = filterItems(rows, func(it spindle.QueueItem) bool { return strings.EqualFold(it.Status, "failed") })
	case filterReview:
		rows = filterItems(rows, func(it spindle.QueueItem) bool { return it.NeedsReview })
	case filterProcessing:
		rows = filterItems(rows, func(it spindle.QueueItem) bool {
			return isProcessingStatus(it.Status)
		})
	}
	if vm.queueSearch.regex != nil {
		rows = filterItems(rows, func(it spindle.QueueItem) bool {
			return vm.queueSearch.regex.MatchString(vm.queueSearchHaystack(it))
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].NeedsReview != rows[j].NeedsReview {
			return rows[i].NeedsReview
		}
		pi := statusRank(rows[i].Status)
		pj := statusRank(rows[j].Status)
		if pi != pj {
			return pi < pj
		}
		ti := mostRecentTimestamp(rows[i])
		tj := mostRecentTimestamp(rows[j])
		if ti.IsZero() && !tj.IsZero() {
			return false
		}
		if tj.IsZero() && !ti.IsZero() {
			return true
		}
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return rows[i].ID > rows[j].ID
	})

	vm.updateQueueTitle(len(rows), len(vm.items))

	for itemIdx, item := range rows {
		row := itemToRow(itemIdx)
		vm.table.SetCell(row, 0, vm.makeCell(vm.formatQueueRow(item, false), tview.AlignLeft, 1))
	}

	vm.displayItems = rows
}

func (vm *viewModel) updateQueueTitle(visible, total int) {
	count := fmt.Sprintf("(%d)", visible)
	if total > 0 && visible != total {
		count = fmt.Sprintf("(%d/%d)", visible, total)
	}

	parts := []string{
		fmt.Sprintf("[::b]Queue[::-] [%s]%s[-]", vm.theme.Text.Muted, count),
	}

	filterLabel := ""
	filterColor := vm.theme.Text.Muted
	switch vm.filterMode {
	case filterFailed:
		filterLabel = "Failed"
		filterColor = vm.theme.Text.Danger
	case filterReview:
		filterLabel = "Review"
		filterColor = vm.theme.Text.Warning
	case filterProcessing:
		filterLabel = "Processing"
		filterColor = vm.colorForStatus("encoding")
	}
	if filterLabel != "" {
		parts = append(parts, fmt.Sprintf("[%s::b]%s[-]", filterColor, strings.ToUpper(filterLabel)))
	}

	if pattern := strings.TrimSpace(vm.queueSearch.pattern); pattern != "" {
		pattern = truncate(pattern, 18)
		parts = append(parts, fmt.Sprintf("[%s]/%s[-]", vm.theme.Text.AccentSoft, tview.Escape(pattern)))
	}

	sep := fmt.Sprintf(" [%s]•[-] ", vm.theme.Text.Faint)
	vm.table.SetTitle(" " + strings.Join(parts, sep) + " ")
}

func (vm *viewModel) renderTablePreservingSelection() {
	selectedID := int64(0)
	if item := vm.selectedItem(); item != nil {
		selectedID = item.ID
	}

	vm.renderTable()

	itemCount := len(vm.displayItems)
	if itemCount <= 0 {
		return
	}

	if selectedID != 0 {
		if itemIdx := vm.findItemIndexByID(selectedID); itemIdx >= 0 {
			vm.table.Select(itemToRow(itemIdx), 0)
			vm.applySelectionStyling()
			return
		}
		vm.table.Select(itemToRow(0), 0)
		vm.applySelectionStyling()
		return
	}

	row, _ := vm.table.GetSelection()
	itemIdx := rowToItem(row)
	if itemIdx < 0 {
		vm.table.Select(itemToRow(0), 0)
	} else if itemIdx >= itemCount {
		vm.table.Select(itemToRow(itemCount-1), 0)
	}
	vm.applySelectionStyling()
}

// findItemIndexByID returns the index of the item with the given ID, or -1 if not found.
func (vm *viewModel) findItemIndexByID(id int64) int {
	for i, item := range vm.displayItems {
		if item.ID == id {
			return i
		}
	}
	return -1
}

// applySelectionStyling applies visual highlighting to the selected row.
func (vm *viewModel) applySelectionStyling() {
	row, _ := vm.table.GetSelection()
	selectedIdx := rowToItem(row)

	selBg := vm.theme.TableSelectionBackground()
	surfaceBg := vm.theme.SurfaceColor()

	for itemIdx, item := range vm.displayItems {
		r := itemToRow(itemIdx)
		isSelected := itemIdx == selectedIdx

		bg := surfaceBg
		if isSelected {
			bg = selBg
		}

		vm.table.GetCell(r, 0).SetText(vm.formatQueueRow(item, isSelected)).SetBackgroundColor(bg)
	}
}

func (vm *viewModel) makeCell(content string, align, expansion int) *tview.TableCell {
	return tview.NewTableCell(content).
		SetAlign(align).
		SetExpansion(expansion).
		SetBackgroundColor(vm.theme.SurfaceColor())
}

func filterItems(items []spindle.QueueItem, keep func(spindle.QueueItem) bool) []spindle.QueueItem {
	out := items[:0]
	for _, it := range items {
		if keep(it) {
			out = append(out, it)
		}
	}
	return out
}

// formatQueueRow formats "#ID Title · Stage Progress%" for the queue list.
func (vm *viewModel) formatQueueRow(item spindle.QueueItem, selected bool) string {
	title := composeTitle(item)
	stage := titleCase(effectiveQueueStage(item))

	statusParts := []string{stage}
	if isProcessingStatus(item.Status) && item.Progress.Percent > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%.0f%%", min(item.Progress.Percent, 100)))
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		statusParts = append(statusParts, "!")
	}
	if item.NeedsReview {
		statusParts = append(statusParts, "R")
	}
	status := strings.Join(statusParts, " ")

	if selected {
		selColor := vm.theme.TableSelectionTextHex()
		return fmt.Sprintf("[%s]#%d %s · %s[-]", selColor, item.ID, tview.Escape(title), status)
	}

	// Normal colors
	titleColor := vm.theme.Text.Primary
	if item.NeedsReview {
		titleColor = vm.theme.Badges.Review
	}

	idPart := fmt.Sprintf("[%s]#%d[-]", vm.theme.Text.Muted, item.ID)
	titlePart := fmt.Sprintf("[%s]%s[-]", titleColor, tview.Escape(title))
	statusPart := fmt.Sprintf("[%s]%s[-]", vm.colorForStatus(item.Status), status)
	sep := fmt.Sprintf(" [%s]·[-] ", vm.theme.Text.Faint)

	return idPart + " " + titlePart + sep + statusPart
}

func (vm *viewModel) colorForStatus(status string) string {
	return vm.theme.StatusColor(status)
}

func (vm *viewModel) statusChip(status string) string {
	color := vm.colorForStatus(status)
	text := strings.ToUpper(titleCase(status))
	if text == "" {
		text = "UNKNOWN"
	}
	return fmt.Sprintf("[%s:%s] %s [-:-]", vm.theme.Base.Background, color, tview.Escape(text))
}

func (vm *viewModel) laneChip(lane string) string {
	l := strings.ToLower(strings.TrimSpace(lane))
	if l == "" {
		return ""
	}
	return fmt.Sprintf("[%s:%s] %s [-:-]", vm.theme.Base.Background, vm.colorForLane(l), tview.Escape(strings.ToUpper(l)))
}

func (vm *viewModel) badge(text, color string) string {
	fg := vm.theme.Base.Surface
	if strings.TrimSpace(fg) == "" {
		fg = vm.theme.Base.Background
	}
	return fmt.Sprintf("[%s:%s] %s [-:-]", color, fg, text)
}

func (vm *viewModel) colorForLane(lane string) string {
	return vm.theme.LaneColor(lane)
}

func mostRecentTimestamp(item spindle.QueueItem) time.Time {
	updated := item.ParsedUpdatedAt()
	created := item.ParsedCreatedAt()
	if updated.After(created) {
		return updated
	}
	return created
}

func composeTitle(item spindle.QueueItem) string {
	title := strings.TrimSpace(item.DiscTitle)
	if title != "" {
		return title
	}
	return fallbackTitle(item.SourcePath)
}

func fallbackTitle(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "Unknown"
	}
	parts := strings.Split(trimmed, "/")
	name := parts[len(parts)-1]
	if name == "" && len(parts) > 1 {
		return parts[len(parts)-2]
	}
	return name
}

func determineLane(item spindle.QueueItem) string {
	lane := strings.ToLower(strings.TrimSpace(item.ProcessingLane))
	switch lane {
	case "foreground", "background":
		return lane
	}

	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case "pending", "identifying", "identified", "ripping":
		return "foreground"
	case "ripped", "episode_identifying", "episode_identified", "encoding", "encoded", "subtitling", "subtitled", "organizing", "completed":
		return "background"
	case "failed":
		return "attention"
	default:
		return ""
	}
}
