package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (vm *viewModel) showHelp() {
	helpSections := []struct {
		title   string
		entries []struct{ key, desc string }
	}{
		{
			title: "Navigation",
			entries: []struct{ key, desc string }{
				{"Tab", "Cycle pane focus (Queue ↔ Detail ↔ Logs)"},
				{"ESC", "Return focus to queue"},
				{"q", "Jump to queue table"},
			},
		},
		{
			title: "Logs & Details",
			entries: []struct{ key, desc string }{
				{"l", "Rotate log source (Daemon ↔ Item)"},
				{"i", "Open highlighted item's logs"},
				{"p", "Toggle problems drawer"},
			},
		},
		{
			title: "Search & Filters",
			entries: []struct{ key, desc string }{
				{"/", "Start a new search"},
				{"n / N", "Next / previous match"},
				{"f", "Cycle queue filter (All → Active → Failed)"},
				{"1-9", "Jump to matching problem shortcut"},
			},
		},
		{
			title: "System",
			entries: []struct{ key, desc string }{
				{"h or ?", "Show this help"},
				{"e", "Exit Flyer"},
				{"Ctrl+C", "Quit application"},
			},
		},
	}

	keyColor := hexToColor(vm.theme.Text.AccentSoft)
	descColor := hexToColor(vm.theme.Text.Primary)
	headerColor := hexToColor(vm.theme.Text.Secondary)
	mutedColor := hexToColor(vm.theme.Text.Muted)

	table := tview.NewTable()
	table.SetBorders(false)
	table.SetBackgroundColor(vm.theme.SurfaceColor())
	table.SetSelectable(false, false)
	table.SetFixed(0, 0)
	table.SetEvaluateAllRows(true)

	row := 0
	for idx, section := range helpSections {
		header := tview.NewTableCell(fmt.Sprintf(" %s ", section.title)).
			SetTextColor(headerColor).
			SetAttributes(tcell.AttrBold).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)
		table.SetCell(row, 0, header)
		table.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
		row++
		for _, entry := range section.entries {
			keyCell := tview.NewTableCell(entry.key).
				SetTextColor(keyColor).
				SetAlign(tview.AlignRight).
				SetSelectable(false).
				SetAttributes(tcell.AttrBold)
			descCell := tview.NewTableCell(entry.desc).
				SetTextColor(descColor).
				SetAlign(tview.AlignLeft).
				SetSelectable(false).
				SetExpansion(1)
			table.SetCell(row, 0, keyCell)
			table.SetCell(row, 1, descCell)
			row++
		}
		if idx < len(helpSections)-1 {
			table.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
			row++
		}
	}

	hint := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[%s]Press Esc, Enter, or q to close", vm.theme.Text.Muted)).
		SetTextAlign(tview.AlignCenter)
	hint.SetBackgroundColor(vm.theme.SurfaceColor())
	hint.SetTextColor(mutedColor)

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	content.SetBorder(true).
		SetTitle(" [::b]Flyer Shortcuts[::-] ").
		SetBorderColor(vm.theme.BorderFocusColor()).
		SetBackgroundColor(vm.theme.SurfaceColor())
	content.AddItem(table, 0, 1, true)
	content.AddItem(hint, 1, 0, false)

	closeModal := func() {
		vm.root.RemovePage("modal")
		vm.returnToCurrentView()
	}

	content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEsc,
			event.Key() == tcell.KeyEnter,
			event.Key() == tcell.KeyCtrlC,
			strings.EqualFold(string(event.Rune()), "q"):
			closeModal()
			return nil
		}
		return event
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEsc,
			event.Key() == tcell.KeyEnter,
			event.Key() == tcell.KeyCtrlC,
			event.Rune() == 'q',
			event.Rune() == 'Q':
			closeModal()
			return nil
		}
		return event
	})

	height := row + 6
	if height < 12 {
		height = 12
	}
	if height > 24 {
		height = 24
	}

	vm.root.RemovePage("modal")
	vm.root.AddPage("modal", center(80, height, content), true, true)
	vm.app.SetFocus(table)
}

func center(width, height int, primitive tview.Primitive) tview.Primitive {
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(primitive, width, 0, true).
			AddItem(nil, 0, 1, false), height, 0, true).
		AddItem(nil, 0, 1, false)
}

func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Second:
		return "now"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	case d < 7*24*time.Hour:
		day := int(d.Hours()) / 24
		h := int(d.Hours()) % 24
		if h == 0 {
			return fmt.Sprintf("%dd", day)
		}
		return fmt.Sprintf("%dd %dh", day, h)
	default:
		day := int(d.Hours()) / 24
		return fmt.Sprintf("%dd", day)
	}
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func truncateMiddle(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || value == "" {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	keep := limit - 1 // room for ellipsis rune
	prefix := keep / 2
	suffix := keep - prefix
	return string(runes[:prefix]) + "…/" + string(runes[len(runes)-suffix:])
}

func formatBytes(value int64) string {
	const (
		kiB = 1024
		miB = kiB * 1024
		giB = miB * 1024
	)
	switch {
	case value >= giB:
		return fmt.Sprintf("%.2f GiB", float64(value)/float64(giB))
	case value >= miB:
		return fmt.Sprintf("%.2f MiB", float64(value)/float64(miB))
	case value >= kiB:
		return fmt.Sprintf("%.2f KiB", float64(value)/float64(kiB))
	default:
		return fmt.Sprintf("%d B", value)
	}
}
