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

	vm.queueSearchMode = true
	vm.queueSearchInput = tview.NewInputField()
	vm.queueSearchInput.SetLabel("/")
	vm.queueSearchInput.SetFieldWidth(40)
	vm.queueSearchInput.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.queueSearchInput.SetFieldBackgroundColor(vm.theme.SurfaceAltColor())
	vm.queueSearchInput.SetFieldTextColor(hexToColor(vm.theme.Text.Primary))

	searchContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	searchContainer.SetBackgroundColor(vm.theme.SurfaceColor())
	searchContainer.AddItem(nil, 0, 1, false)
	searchContainer.AddItem(vm.queueSearchInput, 1, 0, true)

	vm.queueSearchInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			vm.performQueueSearch()
		case tcell.KeyESC:
			vm.cancelQueueSearch()
		}
	})

	vm.root.RemovePage("queue-search")
	vm.root.AddPage("queue-search", searchContainer, true, true)
	vm.app.SetFocus(vm.queueSearchInput)
}

func (vm *viewModel) performQueueSearch() {
	if vm.queueSearchInput == nil {
		return
	}
	searchText := strings.TrimSpace(vm.queueSearchInput.GetText())
	if searchText == "" {
		vm.cancelQueueSearch()
		return
	}

	regex, err := regexp.Compile("(?i)" + searchText)
	if err != nil {
		vm.cancelQueueSearch()
		return
	}

	vm.queueSearchRegex = regex
	vm.queueSearchPattern = searchText
	vm.root.RemovePage("queue-search")
	vm.queueSearchMode = false

	vm.renderTable()
	vm.ensureSelection()
	vm.setCommandBar(vm.currentCommandView())
	vm.returnToCurrentView()
}

func (vm *viewModel) cancelQueueSearch() {
	vm.root.RemovePage("queue-search")
	vm.queueSearchMode = false
	vm.returnToCurrentView()
	vm.setCommandBar(vm.currentCommandView())
}

func (vm *viewModel) clearQueueSearch() {
	vm.queueSearchRegex = nil
	vm.queueSearchPattern = ""
	vm.renderTable()
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
