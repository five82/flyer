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

var statusPalette = map[string]string{
	"pending":     "#7f8c8d",
	"identifying": "#74b9ff",
	"identified":  "#74b9ff",
	"ripping":     "#0abde3",
	"ripped":      "#38ada9",
	"encoding":    "#f6c90e",
	"encoded":     "#2ecc71",
	"organizing":  "#27ae60",
	"completed":   "#27ae60",
	"failed":      "#ff6b6b",
	"review":      "#f39c12",
}

var lanePalette = map[string]string{
	"foreground": "#74b9ff",
	"background": "#5b7083",
	"attention":  "#f39c12",
}

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
		{"Title", tview.AlignLeft, 4},
		{"Stage", tview.AlignLeft, 3},
		{"Progress", tview.AlignLeft, 3},
		{"Status", tview.AlignLeft, 2},
		{"Updated", tview.AlignLeft, 2},
		{"Flags", tview.AlignLeft, 1},
	}

	headerBackground := tcell.ColorRoyalBlue
	for col, column := range columns {
		header := tview.NewTableCell(fmt.Sprintf("[#f1f5f9::b]%s[-]", column.label)).
			SetSelectable(false).
			SetAlign(column.align).
			SetExpansion(column.expansion).
			SetBackgroundColor(headerBackground)
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

	now := time.Now()

	for rowIdx, item := range rows {
		displayRow := rowIdx + 1
		vm.table.SetCell(displayRow, 0, makeCell(gutterMarker(item), tview.AlignCenter, 1))
		vm.table.SetCell(displayRow, 1, makeCell(fmt.Sprintf("[#94a3b8]%d[-]", item.ID), tview.AlignRight, 1))
		vm.table.SetCell(displayRow, 2, makeCell(formatTitle(item), tview.AlignLeft, 4))
		vm.table.SetCell(displayRow, 3, makeCell(formatStage(item), tview.AlignLeft, 3))
		vm.table.SetCell(displayRow, 4, makeCell(formatProgressBar(item), tview.AlignLeft, 3))
		vm.table.SetCell(displayRow, 5, makeCell(formatStatus(item), tview.AlignLeft, 2))
		vm.table.SetCell(displayRow, 6, makeCell(formatUpdated(now, item), tview.AlignLeft, 2))
		vm.table.SetCell(displayRow, 7, makeCell(formatFlags(item), tview.AlignLeft, 1))
	}

	vm.displayItems = rows
	vm.table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorSteelBlue).Foreground(tcell.ColorWhite))
}

func makeCell(content string, align, expansion int) *tview.TableCell {
	return tview.NewTableCell(content).
		SetAlign(align).
		SetExpansion(expansion).
		SetBackgroundColor(tcell.ColorBlack)
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

func formatTitle(item spindle.QueueItem) string {
	title := composeTitle(item)
	title = truncate(title, 50)
	color := "#e2e8f0"
	if item.NeedsReview {
		color = "#f39c12"
	}
	return fmt.Sprintf("[%s]%s[-]", color, tview.Escape(title))
}

func gutterMarker(item spindle.QueueItem) string {
	if strings.TrimSpace(item.ErrorMessage) != "" {
		return badge("!", "#ff6b6b")
	}
	if item.NeedsReview {
		return badge("R", "#f39c12")
	}
	return ""
}

func formatStage(item spindle.QueueItem) string {
	stage := strings.TrimSpace(item.Progress.Stage)
	if stage == "" {
		stage = item.Status
	}
	stage = titleCase(stage)
	stage = truncate(stage, 22)

	detail := strings.TrimSpace(item.Progress.Message)
	detailColor := "#9aa5b1"

	if strings.TrimSpace(item.ErrorMessage) != "" {
		detail = item.ErrorMessage
		detailColor = "#ff6b6b"
	} else if item.NeedsReview && strings.TrimSpace(item.ReviewReason) != "" {
		detail = item.ReviewReason
		detailColor = "#f39c12"
	}

	if detail != "" {
		detail = truncate(detail, 36)
		return fmt.Sprintf("[%s]%s[-] [#6c757d]·[-] [%s]%s[-]", colorForStatus(item.Status), tview.Escape(stage), detailColor, tview.Escape(detail))
	}

	return fmt.Sprintf("[%s]%s[-]", colorForStatus(item.Status), tview.Escape(stage))
}

func formatProgressBar(item spindle.QueueItem) string {
	percent := item.Progress.Percent
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	const barWidth = 14
	filled := int(percent/100*barWidth + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := "[" + strings.Repeat("=", filled) + strings.Repeat(".", barWidth-filled) + "]"
	return fmt.Sprintf("[%s]%s[-] %3.0f%%", colorForStatus(item.Status), bar, percent)
}

func formatStatus(item spindle.QueueItem) string {
	status := titleCase(item.Status)
	if status == "" {
		status = "Unknown"
	}

	lane := strings.TrimSpace(item.ProcessingLane)
	if lane == "" {
		lane = determineLane(item.Status)
	}
	laneLower := strings.ToLower(strings.TrimSpace(lane))
	chip := statusChip(item.Status)
	if laneLower != "" {
		laneLabel := titleCase(laneLower)
		return fmt.Sprintf("%s [#6c757d]·[-] [%s]%s[-]", chip, colorForLane(laneLower), tview.Escape(laneLabel))
	}
	return chip
}

func formatUpdated(now time.Time, item spindle.QueueItem) string {
	ts := mostRecentTimestamp(item)
	if ts.IsZero() {
		return "[#94a3b8]-[-]"
	}
	diff := now.Sub(ts)
	if diff < 0 {
		diff = 0
	}
	switch {
	case diff < time.Minute:
		return "[#94a3b8]just now[-]"
	case diff < time.Hour:
		return fmt.Sprintf("[#94a3b8]%dm ago[-]", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("[#94a3b8]%dh ago[-]", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("[#94a3b8]%dd ago[-]", int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / (24 * 7))
		if weeks < 1 {
			weeks = 1
		}
		return fmt.Sprintf("[#94a3b8]%dw ago[-]", weeks)
	default:
		return fmt.Sprintf("[#94a3b8]%s[-]", ts.Format("Jan 02"))
	}
}

func formatFlags(item spindle.QueueItem) string {
	var flags []string
	if item.NeedsReview {
		flags = append(flags, badge("REV", "#f39c12"))
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		flags = append(flags, badge("ERR", "#ff6b6b"))
	}
	if strings.TrimSpace(item.BackgroundLogPath) != "" {
		flags = append(flags, badge("LOG", "#74b9ff"))
	}
	return strings.Join(flags, " ")
}

func colorForStatus(status string) string {
	if color, ok := statusPalette[strings.ToLower(strings.TrimSpace(status))]; ok {
		return color
	}
	return "#cbd5f5"
}

func statusChip(status string) string {
	color := colorForStatus(status)
	text := strings.ToUpper(titleCase(status))
	if text == "" {
		text = "UNKNOWN"
	}
	return fmt.Sprintf("[black:%s] %s [-:-]", color, tview.Escape(text))
}

func badge(text, color string) string {
	return fmt.Sprintf("[black:%s] %s [-:-]", color, text)
}

func colorForLane(lane string) string {
	if color, ok := lanePalette[strings.ToLower(strings.TrimSpace(lane))]; ok {
		return color
	}
	return "#94a3b8"
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
