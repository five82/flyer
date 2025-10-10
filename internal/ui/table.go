package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

func (vm *viewModel) renderTable() {
	vm.table.Clear()
	headers := []string{"ID", "Title", "Status", "Lane", "Progress"}
	for col, label := range headers {
		// k9s-style table headers with white background
		headerCell := tview.NewTableCell("[::b][black:white]" + label + "[-]")
		headerCell.SetSelectable(false)
		vm.table.SetCell(0, col, headerCell)
	}

	rows := vm.items
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].ID > rows[j].ID
	})

	for row := 0; row < len(rows); row++ {
		item := rows[row]
		// k9s-style table data with dodgerblue color
		vm.table.SetCell(row+1, 0, tview.NewTableCell(fmt.Sprintf("[dodgerblue]%d[-]", item.ID)))
		vm.table.SetCell(row+1, 1, tview.NewTableCell("[dodgerblue]"+composeTitle(item)+"[-]"))
		vm.table.SetCell(row+1, 2, tview.NewTableCell("[dodgerblue]"+strings.ToUpper(item.Status)+"[-]"))
		vm.table.SetCell(row+1, 3, tview.NewTableCell("[dodgerblue]"+determineLane(item.Status)+"[-]"))
		vm.table.SetCell(row+1, 4, tview.NewTableCell("[dodgerblue]"+formatProgress(item)+"[-]"))
	}

	// Set k9s-style selection colors
	vm.table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorAqua).Foreground(tcell.ColorBlack))
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

func formatProgress(item spindle.QueueItem) string {
	stage := strings.TrimSpace(item.Progress.Stage)
	if stage == "" {
		stage = titleCase(item.Status)
	}
	percent := item.Progress.Percent
	if percent <= 0 {
		return stage
	}
	return fmt.Sprintf("%s %.0f%%", stage, percent)
}

func determineLane(status string) string {
	switch status {
	case "pending", "identifying", "identified", "ripping":
		return "foreground"
	case "ripped", "encoding", "encoded", "organizing", "completed":
		return "background"
	case "failed", "review":
		return "attention"
	default:
		return "-"
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
