package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
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

// rowsPerItem defines how many table rows each queue item occupies.
const rowsPerItem = 2

// itemToFirstRow converts an item index (0-based) to the first table row for that item.
// Row 0 is the header, so item 0 starts at row 1.
func itemToFirstRow(itemIdx int) int {
	return 1 + itemIdx*rowsPerItem
}

// rowToItem converts a table row to the item index.
// Returns -1 if the row is the header (row 0).
func rowToItem(row int) int {
	if row <= 0 {
		return -1
	}
	return (row - 1) / rowsPerItem
}

// tableColumn defines a column in the queue table.
type tableColumn struct {
	label     string
	align     int
	expansion int
}

func (vm *viewModel) renderTable() {
	vm.table.Clear()

	_, _, width, _ := vm.table.GetInnerRect()
	if width <= 0 {
		width = 120
	}
	showUpdated := width >= LayoutUpdatedWidth

	// Multi-line layout: 3 columns - gutter, content, right
	columns := []tableColumn{
		{"", tview.AlignCenter, 1},    // gutter
		{"Title", tview.AlignLeft, 8}, // main content
		{"", tview.AlignRight, 2},     // flags/updated
	}

	headerBackground := vm.theme.TableHeaderBackground()
	headerText := vm.theme.TableHeaderTextColor()
	for col, column := range columns {
		header := tview.NewTableCell(fmt.Sprintf("[%s::b]%s[-]", vm.theme.Text.Heading, column.label)).
			SetSelectable(false).
			SetAlign(column.align).
			SetExpansion(column.expansion).
			SetBackgroundColor(headerBackground).
			SetTextColor(headerText)
		vm.table.SetCell(0, col, header)
	}

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

	// Title limit increased since it gets full width
	titleLimit := 60
	if width < LayoutCompactWidth {
		titleLimit = 40
	}

	now := time.Now()
	for itemIdx, item := range rows {
		row1 := itemToFirstRow(itemIdx)
		row2 := row1 + 1

		// Row 1: [gutter] [#ID Title] [Flags]
		vm.table.SetCell(row1, 0, vm.makeCell(vm.gutterMarker(item), tview.AlignCenter, 1))
		vm.table.SetCell(row1, 1, vm.makeCell(vm.formatTitleWithID(item, titleLimit), tview.AlignLeft, 8))
		vm.table.SetCell(row1, 2, vm.makeCell(vm.formatFlags(item), tview.AlignRight, 2))

		// Row 2: [spacer] [Stage · Progress Detail] [Updated]
		vm.table.SetCell(row2, 0, vm.makeCell("", tview.AlignCenter, 1))
		vm.table.SetCell(row2, 1, vm.makeCell(vm.formatStageWithDetail(item), tview.AlignLeft, 8))
		updatedText := ""
		if showUpdated {
			updatedText = vm.formatUpdated(now, item)
		}
		vm.table.SetCell(row2, 2, vm.makeCell(updatedText, tview.AlignRight, 2))
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
			vm.table.Select(itemToFirstRow(itemIdx), 0)
			vm.applySelectionStyling()
			return
		}
		vm.table.Select(itemToFirstRow(0), 0)
		vm.applySelectionStyling()
		return
	}

	row, _ := vm.table.GetSelection()
	itemIdx := rowToItem(row)
	if itemIdx < 0 {
		vm.table.Select(itemToFirstRow(0), 0)
	} else if itemIdx >= itemCount {
		vm.table.Select(itemToFirstRow(itemCount-1), 0)
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

// applySelectionStyling applies visual highlighting to both rows of the selected item.
func (vm *viewModel) applySelectionStyling() {
	row, _ := vm.table.GetSelection()
	selectedIdx := rowToItem(row)

	_, _, width, _ := vm.table.GetInnerRect()
	if width <= 0 {
		width = 120
	}
	showUpdated := width >= LayoutUpdatedWidth
	titleLimit := 60
	if width < LayoutCompactWidth {
		titleLimit = 40
	}

	now := time.Now()
	selBg := vm.theme.TableSelectionBackground()
	surfaceBg := vm.theme.SurfaceColor()

	// Re-render all items, applying selection styling to the selected one
	for itemIdx, item := range vm.displayItems {
		row1 := itemToFirstRow(itemIdx)
		row2 := row1 + 1
		isSelected := itemIdx == selectedIdx

		var bg tcell.Color
		if isSelected {
			bg = selBg
		} else {
			bg = surfaceBg
		}

		// Row 1: [gutter] [#ID Title] [Flags]
		vm.table.GetCell(row1, 0).SetText(vm.gutterMarkerStyled(item, isSelected)).SetBackgroundColor(bg)
		vm.table.GetCell(row1, 1).SetText(vm.formatTitleWithIDStyled(item, titleLimit, isSelected)).SetBackgroundColor(bg)
		vm.table.GetCell(row1, 2).SetText(vm.formatFlagsStyled(item, isSelected)).SetBackgroundColor(bg)

		// Row 2: [spacer] [Stage · Progress Detail] [Updated]
		vm.table.GetCell(row2, 0).SetText("").SetBackgroundColor(bg)
		vm.table.GetCell(row2, 1).SetText(vm.formatStageWithDetailStyled(item, isSelected)).SetBackgroundColor(bg)
		updatedText := ""
		if showUpdated {
			updatedText = vm.formatUpdatedStyled(now, item, isSelected)
		}
		vm.table.GetCell(row2, 2).SetText(updatedText).SetBackgroundColor(bg)
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

// formatTitleWithID formats the title with ID prefix for the multi-line layout.
func (vm *viewModel) formatTitleWithID(item spindle.QueueItem, limit int) string {
	title := composeTitle(item)
	title = truncate(title, limit)
	color := vm.theme.Text.Primary
	if item.NeedsReview {
		color = vm.theme.Badges.Review
	}
	idPrefix := fmt.Sprintf("[%s]#%d[-] ", vm.theme.Text.Muted, item.ID)
	return idPrefix + fmt.Sprintf("[%s]%s[-]", color, tview.Escape(title))
}

// formatStageWithDetail formats stage and progress detail for the second row.
func (vm *viewModel) formatStageWithDetail(item spindle.QueueItem) string {
	stage := effectiveQueueStage(item)
	stage = titleCase(stage)

	var parts []string
	parts = append(parts, fmt.Sprintf("[%s]%s[-]", vm.colorForStatus(item.Status), tview.Escape(stage)))

	// Add compact progress percentage if actively processing
	if isProcessingStatus(item.Status) && item.Progress.Percent > 0 {
		percent := item.Progress.Percent
		if percent > 100 {
			percent = 100
		}
		parts = append(parts, fmt.Sprintf("[%s]%3.0f%%[-]", vm.colorForStatus(item.Status), percent))
	}

	// Add detail message
	detail := strings.TrimSpace(item.Progress.Message)
	detailColor := vm.theme.Text.Muted

	if strings.TrimSpace(item.ErrorMessage) != "" {
		detail = item.ErrorMessage
		detailColor = vm.theme.Text.Danger
	} else if item.NeedsReview && strings.TrimSpace(item.ReviewReason) != "" {
		detail = item.ReviewReason
		detailColor = vm.theme.Text.Warning
	}

	if detail != "" {
		detail = truncate(detail, 50)
		parts = append(parts, fmt.Sprintf("[%s]%s[-]", detailColor, tview.Escape(detail)))
	}

	sep := fmt.Sprintf(" [%s]·[-] ", vm.theme.Text.Faint)
	return strings.Join(parts, sep)
}

func (vm *viewModel) gutterMarker(item spindle.QueueItem) string {
	if strings.TrimSpace(item.ErrorMessage) != "" {
		return vm.badge("!", vm.theme.Badges.Error)
	}
	if item.NeedsReview {
		return vm.badge("R", vm.theme.Badges.Review)
	}
	return ""
}

func (vm *viewModel) formatStage(item spindle.QueueItem) string {
	stage := effectiveQueueStage(item)
	stage = titleCase(stage)
	stage = truncate(stage, 22)

	detail := strings.TrimSpace(item.Progress.Message)
	detailColor := vm.theme.Text.Muted

	if strings.TrimSpace(item.ErrorMessage) != "" {
		detail = item.ErrorMessage
		detailColor = vm.theme.Text.Danger
	} else if item.NeedsReview && strings.TrimSpace(item.ReviewReason) != "" {
		detail = item.ReviewReason
		detailColor = vm.theme.Text.Warning
	}

	if detail != "" {
		detail = truncate(detail, 36)
		return fmt.Sprintf("[%s]%s[-] [%s]·[-] [%s]%s[-]", vm.colorForStatus(item.Status), tview.Escape(stage), vm.theme.Text.Faint, detailColor, tview.Escape(detail))
	}

	return fmt.Sprintf("[%s]%s[-]", vm.colorForStatus(item.Status), tview.Escape(stage))
}

func (vm *viewModel) formatUpdated(now time.Time, item spindle.QueueItem) string {
	ts := mostRecentTimestamp(item)
	if ts.IsZero() {
		return fmt.Sprintf("[%s]-[-]", vm.theme.Text.Muted)
	}
	diff := now.Sub(ts)
	if diff < 0 {
		diff = 0
	}
	color := vm.theme.Text.Muted
	switch {
	case diff < time.Minute:
		return fmt.Sprintf("[%s]just now[-]", color)
	case diff < time.Hour:
		return fmt.Sprintf("[%s]%dm ago[-]", color, int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("[%s]%dh ago[-]", color, int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("[%s]%dd ago[-]", color, int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / (24 * 7))
		if weeks < 1 {
			weeks = 1
		}
		return fmt.Sprintf("[%s]%dw ago[-]", color, weeks)
	default:
		return fmt.Sprintf("[%s]%s[-]", color, ts.In(time.Local).Format("Jan 02"))
	}
}

func (vm *viewModel) formatFlags(item spindle.QueueItem) string {
	var flags []string
	if item.NeedsReview {
		flags = append(flags, vm.badge("REV", vm.theme.Badges.Review))
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		flags = append(flags, vm.badge("ERR", vm.theme.Badges.Error))
	}
	if strings.TrimSpace(item.ItemLogPath) != "" {
		flags = append(flags, vm.badge("LOG", vm.theme.Badges.Log))
	}
	if isRipCacheHitMessage(item.Progress.Message) {
		flags = append(flags, vm.badge("CACHE", vm.theme.Badges.Info))
	}
	if badge := vm.subtitleFallbackBadge(item); badge != "" {
		flags = append(flags, badge)
	}
	if badge := vm.episodeProgressBadge(item); badge != "" {
		flags = append(flags, badge)
	}
	return strings.Join(flags, " ")
}

// Styled variants for selection - use selection text color for better contrast

func (vm *viewModel) gutterMarkerStyled(item spindle.QueueItem, selected bool) string {
	if !selected {
		return vm.gutterMarker(item)
	}
	selColor := vm.theme.TableSelectionTextHex()
	if strings.TrimSpace(item.ErrorMessage) != "" {
		return fmt.Sprintf("[%s::b]![-]", selColor)
	}
	if item.NeedsReview {
		return fmt.Sprintf("[%s::b]R[-]", selColor)
	}
	return ""
}

func (vm *viewModel) formatTitleWithIDStyled(item spindle.QueueItem, limit int, selected bool) string {
	if !selected {
		return vm.formatTitleWithID(item, limit)
	}
	selColor := vm.theme.TableSelectionTextHex()
	title := composeTitle(item)
	title = truncate(title, limit)
	return fmt.Sprintf("[%s]#%d %s[-]", selColor, item.ID, tview.Escape(title))
}

func (vm *viewModel) formatStageWithDetailStyled(item spindle.QueueItem, selected bool) string {
	if !selected {
		return vm.formatStageWithDetail(item)
	}
	selColor := vm.theme.TableSelectionTextHex()

	stage := effectiveQueueStage(item)
	stage = titleCase(stage)

	var parts []string
	parts = append(parts, tview.Escape(stage))

	// Add compact progress percentage if actively processing
	if isProcessingStatus(item.Status) && item.Progress.Percent > 0 {
		percent := item.Progress.Percent
		if percent > 100 {
			percent = 100
		}
		parts = append(parts, fmt.Sprintf("%3.0f%%", percent))
	}

	// Add detail message
	detail := strings.TrimSpace(item.Progress.Message)
	if strings.TrimSpace(item.ErrorMessage) != "" {
		detail = item.ErrorMessage
	} else if item.NeedsReview && strings.TrimSpace(item.ReviewReason) != "" {
		detail = item.ReviewReason
	}

	if detail != "" {
		detail = truncate(detail, 50)
		parts = append(parts, tview.Escape(detail))
	}

	return fmt.Sprintf("[%s]%s[-]", selColor, strings.Join(parts, " · "))
}

func (vm *viewModel) formatUpdatedStyled(now time.Time, item spindle.QueueItem, selected bool) string {
	if !selected {
		return vm.formatUpdated(now, item)
	}
	selColor := vm.theme.TableSelectionTextHex()
	ts := mostRecentTimestamp(item)
	if ts.IsZero() {
		return fmt.Sprintf("[%s]-[-]", selColor)
	}
	diff := now.Sub(ts)
	if diff < 0 {
		diff = 0
	}
	switch {
	case diff < time.Minute:
		return fmt.Sprintf("[%s]just now[-]", selColor)
	case diff < time.Hour:
		return fmt.Sprintf("[%s]%dm ago[-]", selColor, int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("[%s]%dh ago[-]", selColor, int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("[%s]%dd ago[-]", selColor, int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / (24 * 7))
		if weeks < 1 {
			weeks = 1
		}
		return fmt.Sprintf("[%s]%dw ago[-]", selColor, weeks)
	default:
		return fmt.Sprintf("[%s]%s[-]", selColor, ts.In(time.Local).Format("Jan 02"))
	}
}

func (vm *viewModel) formatFlagsStyled(item spindle.QueueItem, selected bool) string {
	if !selected {
		return vm.formatFlags(item)
	}
	// For selected rows, keep the badges as-is since they have their own distinct styling
	return vm.formatFlags(item)
}

func (vm *viewModel) subtitleFallbackBadge(item spindle.QueueItem) string {
	if item.SubtitleGeneration == nil || !item.SubtitleGeneration.FallbackUsed {
		return ""
	}
	label := "AI"
	if item.SubtitleGeneration.WhisperX > 1 {
		label = fmt.Sprintf("AI%d", item.SubtitleGeneration.WhisperX)
	}
	return vm.badge(label, vm.theme.Badges.Fallback)
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

func (vm *viewModel) episodeProgressBadge(item spindle.QueueItem) string {
	episodes, totals := item.EpisodeSnapshot()
	if totals.Planned < 2 {
		return ""
	}

	// Count failed and subtitled episodes
	failed := len(spindle.FilterFailed(episodes))
	subtitled := 0
	for _, ep := range episodes {
		stage := normalizeEpisodeStage(ep.Stage)
		if stage == "subtitled" || stage == "final" {
			subtitled++
		}
	}

	// If there are failed episodes, show that prominently
	if failed > 0 {
		label := fmt.Sprintf("EP %d/%d ✗%d", totals.Final, totals.Planned, failed)
		return vm.badge(label, vm.theme.Text.Danger)
	}

	completed := totals.Final
	color := vm.theme.StatusColor("pending")
	if totals.Final == totals.Planned && totals.Planned > 0 {
		color = vm.theme.StatusColor("completed")
	} else if subtitled > 0 {
		completed = subtitled
		color = vm.theme.StatusColor("subtitled")
	} else if totals.Encoded > 0 {
		completed = totals.Encoded
		color = vm.theme.StatusColor("encoding")
	} else if totals.Ripped > 0 {
		completed = totals.Ripped
		color = vm.theme.StatusColor("ripping")
	}
	label := fmt.Sprintf("EP %d/%d", completed, totals.Planned)
	return vm.badge(label, color)
}

func (vm *viewModel) colorForLane(lane string) string {
	return vm.theme.LaneColor(lane)
}

func mostRecentTimestamp(item spindle.QueueItem) time.Time {
	updated := item.ParsedUpdatedAt()
	created := item.ParsedCreatedAt()
	if updated.IsZero() && created.IsZero() {
		return time.Time{}
	}
	if updated.IsZero() {
		return created
	}
	if created.IsZero() {
		return updated
	}
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
