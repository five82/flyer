package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

var statusPriority = map[string]int{
	"failed":              0,
	"review":              1,
	"subtitling":          2,
	"encoding":            3,
	"organizing":          4,
	"ripping":             5,
	"episode_identifying": 6,
	"identifying":         7,
	"episode_identified":  8,
	"ripped":              9,
	"subtitled":           10,
	"encoded":             11,
	"identified":          12,
	"pending":             13,
	"completed":           14,
}

func (vm *viewModel) renderTable() {
	vm.table.Clear()

	_, _, width, _ := vm.table.GetInnerRect()
	if width <= 0 {
		width = 120
	}
	showProgress := width >= 120
	showUpdated := width >= 150

	columns := []struct {
		label     string
		align     int
		expansion int
	}{
		{"!", tview.AlignCenter, 1}, // gutter marker
		{"ID", tview.AlignRight, 1},
		{"Title", tview.AlignLeft, 7},
		{"Stage", tview.AlignLeft, 7},
	}
	if showProgress {
		columns = append(columns, struct {
			label     string
			align     int
			expansion int
		}{"%", tview.AlignLeft, 4})
	}
	if showUpdated {
		columns = append(columns, struct {
			label     string
			align     int
			expansion int
		}{"Updated", tview.AlignLeft, 3})
	}
	columns = append(columns, struct {
		label     string
		align     int
		expansion int
	}{"Flags", tview.AlignLeft, 2})

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
			status := strings.ToLower(strings.TrimSpace(it.Status))
			return status == "identifying" ||
				status == "ripping" ||
				status == "episode_identifying" ||
				status == "episode_identified" ||
				status == "encoding" ||
				status == "subtitling" ||
				status == "subtitled" ||
				status == "organizing"
		})
	}
	if vm.queueSearchRegex != nil {
		rows = filterItems(rows, func(it spindle.QueueItem) bool {
			return vm.queueSearchRegex.MatchString(vm.queueSearchHaystack(it))
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

	titleLimit := 44
	if showUpdated {
		titleLimit = 34
	} else if showProgress {
		titleLimit = 38
	}
	if width < 100 {
		titleLimit = 28
	}

	now := time.Now()
	for rowIdx, item := range rows {
		displayRow := rowIdx + 1
		col := 0
		vm.table.SetCell(displayRow, col, vm.makeCell(vm.gutterMarker(item), tview.AlignCenter, 1))
		col++
		vm.table.SetCell(displayRow, col, vm.makeCell(fmt.Sprintf("[%s]%d[-]", vm.theme.Text.Muted, item.ID), tview.AlignRight, 1))
		col++
		vm.table.SetCell(displayRow, col, vm.makeCell(vm.formatTitle(item, titleLimit), tview.AlignLeft, 7))
		col++
		vm.table.SetCell(displayRow, col, vm.makeCell(vm.formatStage(item), tview.AlignLeft, 7))
		col++
		if showProgress {
			vm.table.SetCell(displayRow, col, vm.makeCell(vm.formatProgressBar(item), tview.AlignLeft, 4))
			col++
		}
		if showUpdated {
			vm.table.SetCell(displayRow, col, vm.makeCell(vm.formatUpdated(now, item), tview.AlignLeft, 3))
			col++
		}
		vm.table.SetCell(displayRow, col, vm.makeCell(vm.formatFlags(item), tview.AlignLeft, 2))
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

	if pattern := strings.TrimSpace(vm.queueSearchPattern); pattern != "" {
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

	rows := vm.table.GetRowCount() - 1 // exclude header
	if rows <= 0 {
		return
	}

	if selectedID != 0 {
		if row := vm.findRowByID(selectedID); row > 0 {
			vm.table.Select(row, 0)
			return
		}
		vm.table.Select(1, 0)
		return
	}

	row, _ := vm.table.GetSelection()
	if row <= 0 {
		vm.table.Select(1, 0)
		return
	}
	if row > rows {
		vm.table.Select(rows, 0)
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

func (vm *viewModel) formatTitle(item spindle.QueueItem, limit int) string {
	title := composeTitle(item)
	title = truncate(title, limit)
	color := vm.theme.Text.Primary
	if item.NeedsReview {
		color = vm.theme.Badges.Review
	}
	return fmt.Sprintf("[%s]%s[-]", color, tview.Escape(title))
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
	stage := strings.TrimSpace(item.Progress.Stage)
	if stage == "" {
		stage = item.Status
	}
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

func (vm *viewModel) formatProgressBar(item spindle.QueueItem) string {
	percent := item.Progress.Percent
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	const barWidth = 10
	filled := int(percent/100*barWidth + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}

	// Use Unicode blocks for smoother progress display
	blocks := []rune{'█', '▓', '▒', '░'}
	var bar strings.Builder

	// Add full blocks
	if filled > 0 {
		bar.WriteString(fmt.Sprintf("[%s]%s[-]", vm.colorForStatus(item.Status), strings.Repeat(string(blocks[0]), filled)))
	}

	// Add empty space
	remaining := barWidth - filled
	if remaining > 0 {
		bar.WriteString(fmt.Sprintf("[%s]%s[-]", vm.theme.Text.Faint, strings.Repeat(string(blocks[3]), remaining)))
	}

	return fmt.Sprintf("%s %3.0f%%", bar.String(), percent)
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
		return fmt.Sprintf("[%s]%s[-]", color, ts.Format("Jan 02"))
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
	if strings.TrimSpace(item.BackgroundLogPath) != "" {
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
	subtitled := 0
	for _, ep := range episodes {
		if ep.Stage == "subtitled" || ep.Stage == "final" {
			subtitled++
		}
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

func statusRank(status string) int {
	if rank, ok := statusPriority[strings.ToLower(strings.TrimSpace(status))]; ok {
		return rank
	}
	return 999
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
	case "failed", "review":
		return "attention"
	default:
		return ""
	}
}

func titleCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
