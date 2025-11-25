package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"
)

func (vm *viewModel) showHelp() {
	// k9s-style help text with bracketed keys in column layout
	helpCommands := []struct{ key, desc string }{
		{"q", "Queue View"},
		{"l", "Toggle Log Source (Daemon→Encoding→Item)"},
		{"r", "Encoding Log View"},
		{"p", "Problems Drawer"},
		{"h/?", "Help"},
		{"i", "Item Logs (Highlighted)"},
		{"/", "Start New Search"},
		{"n", "Next Search Match"},
		{"N", "Previous Search Match"},
		{"Tab", "Cycle Pane (Queue↔Detail↔Logs)"},
		{"ESC", "Return to Queue View"},
		{"e", "Exit"},
		{"Ctrl+C", "Exit"},
	}

	// Create formatted help text
	var helpLines []string
	maxRows := 4
	for i, cmd := range helpCommands {
		row := i % maxRows
		col := i / maxRows

		text := fmt.Sprintf("[%s]<%s>[%s] %s", vm.theme.Text.AccentSoft, cmd.key, vm.theme.Text.Muted, cmd.desc)
		for len(helpLines) <= row {
			helpLines = append(helpLines, "")
		}
		if col > 0 {
			helpLines[row] += "  |  " + text
		} else {
			helpLines[row] = text
		}
	}

	text := strings.Join(helpLines, "\n")
	modal := tview.NewModal().SetText(text).AddButtons([]string{"Close"})
	modal.SetBorderColor(vm.theme.BorderFocusColor())
	modal.SetBackgroundColor(vm.theme.SurfaceColor())
	modal.SetTextColor(hexToColor(vm.theme.Text.AccentSoft))
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		vm.root.RemovePage("modal")
		vm.returnToCurrentView()
	})
	vm.root.RemovePage("modal")
	vm.root.AddPage("modal", center(75, 7, modal), true, true)
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
