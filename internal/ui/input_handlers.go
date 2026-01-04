package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// makeVimNavHandler creates a standard vim-style navigation handler for text views.
// The onScroll callback is invoked when any scroll action occurs (can be nil).
func makeVimNavHandler(onScroll func()) func(*tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		var result *tcell.EventKey

		switch event.Rune() {
		case 'j':
			result = tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			result = tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case 'g':
			result = tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
		case 'G':
			result = tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
		default:
			return event
		}

		if onScroll != nil {
			onScroll()
		}
		return result
	}
}

// focusablePane represents a pane that can receive focus with its associated command view.
type focusablePane struct {
	primitive      tview.Primitive
	commandView    string
	useTableBg     bool // true for table (uses SurfaceColor), false for others (SurfaceAltColor)
	hasBorderFocus bool // true if this pane has the focus ring
}

// applyFocusStyles updates all pane styles based on which one is focused.
func (vm *viewModel) applyFocusStyles(focusedCommandView string) {
	panes := []focusablePane{
		{vm.table, "queue", true, false},
		{vm.detail, "detail", false, false},
		{vm.logView, "logs", false, false},
		{vm.problemsView, "problems", false, false},
	}

	// Mark the focused pane
	for i := range panes {
		if panes[i].commandView == focusedCommandView {
			panes[i].hasBorderFocus = true
		}
	}

	for _, pane := range panes {
		bordered, ok := pane.primitive.(interface {
			SetBorderColor(tcell.Color) *tview.Box
			SetBackgroundColor(tcell.Color) *tview.Box
		})
		if !ok {
			continue
		}

		if pane.hasBorderFocus {
			bordered.SetBorderColor(vm.theme.TableBorderFocusColor())
			bordered.SetBackgroundColor(vm.theme.FocusBackgroundColor())
		} else {
			bordered.SetBorderColor(vm.theme.TableBorderColor())
			if pane.useTableBg {
				bordered.SetBackgroundColor(vm.theme.SurfaceColor())
			} else {
				bordered.SetBackgroundColor(vm.theme.SurfaceAltColor())
			}
		}
	}

	vm.setCommandBar(focusedCommandView)
}

// setupFocusHandlers configures focus handlers for all panes.
func (vm *viewModel) setupFocusHandlers() {
	vm.table.SetFocusFunc(func() {
		vm.applyFocusStyles("queue")
	})
	vm.detail.SetFocusFunc(func() {
		vm.applyFocusStyles("detail")
	})
	vm.logView.SetFocusFunc(func() {
		vm.applyFocusStyles("logs")
	})
	vm.problemsView.SetFocusFunc(func() {
		vm.applyFocusStyles("problems")
	})
}

// makeLogViewHandler creates a handler for log views with follow mode support.
func (vm *viewModel) makeLogViewHandler() func(*tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		disableFollow := func() {
			if vm.logs.follow {
				vm.logs.follow = false
				vm.updateLogStatus(false, vm.logs.lastPath)
				vm.setCommandBar("logs")
			}
		}

		switch {
		case event.Key() == tcell.KeyUp || event.Key() == tcell.KeyPgUp || event.Key() == tcell.KeyHome:
			disableFollow()
			return event
		case event.Rune() == 'k':
			disableFollow()
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == 'j':
			disableFollow()
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'g':
			disableFollow()
			return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
		case event.Key() == tcell.KeyEnd || event.Rune() == 'G':
			vm.logs.follow = true
			vm.logView.ScrollToEnd()
			vm.refreshLogs(true)
			vm.updateLogStatus(vm.logs.follow, vm.logs.lastPath)
			vm.setCommandBar("logs")
			return nil
		case event.Rune() == ' ':
			vm.logs.follow = !vm.logs.follow
			if vm.logs.follow {
				vm.logView.ScrollToEnd()
				vm.refreshLogs(true)
			}
			vm.updateLogStatus(vm.logs.follow, vm.logs.lastPath)
			vm.setCommandBar("logs")
			return nil
		}
		return event
	}
}
