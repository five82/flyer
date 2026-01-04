package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

func (vm *viewModel) startQueueSearch() {
	if vm.currentView != "queue" {
		vm.showQueueView()
	}

	vm.queueSearch.mode = true
	vm.queueSearch.input = tview.NewInputField()
	vm.queueSearch.input.SetLabel("/")
	vm.queueSearch.input.SetFieldWidth(40)
	vm.queueSearch.input.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.queueSearch.input.SetFieldBackgroundColor(vm.theme.SurfaceAltColor())
	vm.queueSearch.input.SetFieldTextColor(hexToColor(vm.theme.Text.Primary))

	vm.queueSearch.hint = tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	vm.queueSearch.hint.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.queueSearch.hint.SetTextColor(hexToColor(vm.theme.Text.Muted))
	vm.queueSearch.hint.SetText(fmt.Sprintf("[%s]Filter queue (regex, case-insensitive). Enter to apply. Esc to cancel.[-]", vm.theme.Text.Muted))

	vm.queueSearch.input.SetChangedFunc(func(_ string) {
		if vm.queueSearch.hint != nil {
			vm.queueSearch.hint.SetText(fmt.Sprintf("[%s]Filter queue (regex, case-insensitive). Enter to apply. Esc to cancel.[-]", vm.theme.Text.Muted))
		}
	})

	searchContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	searchContainer.SetBackgroundColor(vm.theme.SurfaceColor())
	searchContainer.AddItem(nil, 0, 1, false)
	searchContainer.AddItem(vm.queueSearch.hint, 1, 0, false)
	searchContainer.AddItem(vm.queueSearch.input, 1, 0, true)

	vm.queueSearch.input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			vm.performQueueSearch()
		case tcell.KeyESC:
			vm.cancelQueueSearch()
		}
	})

	vm.root.RemovePage("queue-search")
	vm.root.AddPage("queue-search", searchContainer, true, true)
	vm.app.SetFocus(vm.queueSearch.input)
}

func (vm *viewModel) performQueueSearch() {
	if vm.queueSearch.input == nil {
		return
	}
	searchText := strings.TrimSpace(vm.queueSearch.input.GetText())
	if searchText == "" {
		vm.cancelQueueSearch()
		return
	}

	regex, err := regexp.Compile("(?i)" + searchText)
	if err != nil {
		if vm.queueSearch.hint != nil {
			vm.queueSearch.hint.SetText(fmt.Sprintf("[%s]Invalid regex: %s[-]", vm.theme.Search.Error, tview.Escape(err.Error())))
		}
		return
	}

	vm.queueSearch.regex = regex
	vm.queueSearch.pattern = searchText
	vm.root.RemovePage("queue-search")
	vm.queueSearch.mode = false
	vm.queueSearch.hint = nil

	vm.renderTablePreservingSelection()
	vm.ensureSelection()
	vm.setCommandBar(vm.currentCommandView())
	vm.returnToCurrentView()
}

func (vm *viewModel) cancelQueueSearch() {
	vm.root.RemovePage("queue-search")
	vm.queueSearch.mode = false
	vm.queueSearch.hint = nil
	vm.returnToCurrentView()
	vm.setCommandBar(vm.currentCommandView())
}

func (vm *viewModel) clearQueueSearch() {
	vm.queueSearch.regex = nil
	vm.queueSearch.pattern = ""
	vm.renderTablePreservingSelection()
	vm.ensureSelection()
	vm.setCommandBar(vm.currentCommandView())
	vm.returnToCurrentView()
}

func (vm *viewModel) queueSearchHaystack(item spindle.QueueItem) string {
	title := composeTitle(item)
	parts := []string{
		fmt.Sprintf("%d", item.ID),
		title,
		item.Status,
		item.Progress.Stage,
		item.Progress.Message,
		item.ErrorMessage,
		item.ReviewReason,
		item.SourcePath,
		item.DraptoPresetLabel(),
	}
	return strings.Join(parts, " ")
}

func (vm *viewModel) moveQueueSelection(delta int) {
	rows := vm.table.GetRowCount() - 1
	if rows <= 0 {
		return
	}
	row, _ := vm.table.GetSelection()
	if row <= 0 {
		row = 1
	}
	row += delta
	if row < 1 {
		row = 1
	}
	if row > rows {
		row = rows
	}
	vm.table.Select(row, 0)
}

func (vm *viewModel) selectQueueTop() {
	if vm.table.GetRowCount()-1 <= 0 {
		return
	}
	vm.table.Select(1, 0)
}

func (vm *viewModel) selectQueueBottom() {
	rows := vm.table.GetRowCount() - 1
	if rows <= 0 {
		return
	}
	vm.table.Select(rows, 0)
}
