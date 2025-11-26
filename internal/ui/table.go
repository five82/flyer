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

var statusPriority = map[string]int{
	"failed":      0,
	"review":      1,
	"encoding":    2,
	"ripping":     3,
	"ripped":      4,
	"pending":     5,
	"identifying": 6,
	"identified":  7,
	"organizing":  8,
	"encoded":     9,
	"completed":   10,
}

func (vm *viewModel) renderTable() {
	vm.table.Clear()

	columns := []struct {
		label     string
		align     int
		expansion int
	}{
		{"!", tview.AlignCenter, 1}, // gutter marker
		{"ID", tview.AlignRight, 1},
		{"Title", tview.AlignLeft, 6},
		{"Flags", tview.AlignLeft, 1},
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
			status := strings.ToLower(strings.TrimSpace(it.Status))
			return status == "identifying" || status == "ripping" || status == "encoding" || status == "organizing"
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

	for rowIdx, item := range rows {
		displayRow := rowIdx + 1
		vm.table.SetCell(displayRow, 0, vm.makeCell(vm.gutterMarker(item), tview.AlignCenter, 1))
		vm.table.SetCell(displayRow, 1, vm.makeCell(fmt.Sprintf("[%s]%d[-]", vm.theme.Text.Muted, item.ID), tview.AlignRight, 1))
		vm.table.SetCell(displayRow, 2, vm.makeCell(vm.formatTitle(item), tview.AlignLeft, 6))
		vm.table.SetCell(displayRow, 3, vm.makeCell(vm.formatFlags(item), tview.AlignLeft, 1))
	}

	vm.displayItems = rows
	vm.table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorSteelBlue).Foreground(tcell.ColorWhite))
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

func (vm *viewModel) formatTitle(item spindle.QueueItem) string {
	title := composeTitle(item)
	title = truncate(title, 32)
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
	bar := "[" + strings.Repeat("=", filled) + strings.Repeat(".", barWidth-filled) + "]"
	return fmt.Sprintf("[%s]%s[-] %3.0f%%", vm.colorForStatus(item.Status), bar, percent)
}

func (vm *viewModel) formatStatus(item spindle.QueueItem) string {
	status := titleCase(item.Status)
	if status == "" {
		status = "Unknown"
	}
	return vm.statusChip(item.Status)
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
	if badge := vm.episodeProgressBadge(item); badge != "" {
		flags = append(flags, badge)
	}
	return strings.Join(flags, " ")
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
	return fmt.Sprintf("[%s:%s] %s [-:-]", vm.theme.Base.Background, color, text)
}

func (vm *viewModel) episodeProgressBadge(item spindle.QueueItem) string {
	_, totals := item.EpisodeSnapshot()
	if totals.Planned < 2 {
		return ""
	}
	completed := totals.Final
	color := vm.theme.StatusColor("pending")
	if totals.Final == totals.Planned && totals.Planned > 0 {
		color = vm.theme.StatusColor("completed")
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

func determineLane(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "identifying", "identified", "ripping":
		return "foreground"
	case "ripped", "encoding", "encoded", "organizing", "completed":
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
