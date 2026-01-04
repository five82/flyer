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
				{"Tab", "Cycle focus (Queue → Detail → Item Log → Problems)"},
				{"Shift+Tab", "Cycle focus backward"},
				{"j / k", "Move selection up/down (queue)"},
				{"g / G", "Jump to top/bottom (queue)"},
				{"d", "Focus details pane"},
				{"ESC", "Return to queue"},
				{"q", "Jump to queue"},
			},
		},
		{
			title: "Monitoring",
			entries: []struct{ key, desc string }{
				{"l", "View daemon logs"},
				{"i", "View item logs"},
				{"Space", "Toggle log follow (pause/resume)"},
				{"End / G", "Jump to bottom + follow"},
				{"F", "Filter logs by component"},
				{"p", "View problems"},
			},
		},
		{
			title: "Details & Search",
			entries: []struct{ key, desc string }{
				{"t", "Toggle episodes (details)"},
				{"P", "Toggle path expansion"},
				{"/", "Search queue or logs"},
				{"n / N", "Next/previous search match"},
				{"f", "Cycle queue filter"},
				{"Enter", "Toggle fullscreen view"},
			},
		},
		{
			title: "System",
			entries: []struct{ key, desc string }{
				{"h or ?", "Show this help"},
				{"e", "Exit Flyer"},
				{"Ctrl+C", "Quit immediately"},
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
		SetText(fmt.Sprintf("[%s]Press [%s]Esc[-] or [%s]?[-] to close • [%s]h[-] for help anytime",
			vm.theme.Text.Muted, vm.theme.Text.Accent, vm.theme.Text.Accent, vm.theme.Text.Accent)).
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
			event.Key() == tcell.KeyCtrlC,
			event.Rune() == '?',
			event.Rune() == 'h',
			event.Rune() == 'H':
			closeModal()
			return nil
		}
		return event
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEsc,
			event.Key() == tcell.KeyCtrlC,
			event.Rune() == '?',
			event.Rune() == 'h',
			event.Rune() == 'H':
			closeModal()
			return nil
		}
		return event
	})

	height := row + 8
	if height < 16 {
		height = 16
	}
	if height > 28 {
		height = 28
	}

	vm.root.RemovePage("modal")
	modalW, modalH := vm.modalDimensions(80, height)
	vm.root.AddPage("modal", center(modalW, modalH, content), true, true)
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

func (vm *viewModel) modalDimensions(maxWidth, maxHeight int) (int, int) {
	screenW, screenH := 120, 40
	if vm != nil {
		if vm.mainLayout != nil {
			_, _, w, h := vm.mainLayout.GetRect()
			if w > 0 {
				screenW = w
			}
			if h > 0 {
				screenH = h
			}
		}
		if (screenW <= 0 || screenH <= 0) && vm.root != nil {
			_, _, w, h := vm.root.GetRect()
			if w > 0 {
				screenW = w
			}
			if h > 0 {
				screenH = h
			}
		}
	}

	availW := screenW - 4
	availH := screenH - 4
	if availW < 20 {
		availW = screenW
	}
	if availH < 10 {
		availH = screenH
	}

	width := maxWidth
	if width <= 0 {
		width = availW
	}
	if availW > 0 && width > availW {
		width = availW
	}
	if width < 20 {
		width = 20
	}

	height := maxHeight
	if height <= 0 {
		height = availH
	}
	if availH > 0 && height > availH {
		height = availH
	}
	if height < 7 {
		height = 7
	}

	return width, height
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

func isRipCacheHitMessage(message string) bool {
	msg := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(msg, "rip cache hit")
}
