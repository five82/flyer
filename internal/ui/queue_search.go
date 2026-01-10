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
	vm.queueSearch.input = vm.newThemedInputField("/", 40)

	vm.queueSearch.hint = tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	vm.queueSearch.hint.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.queueSearch.hint.SetTextColor(hexToColor(vm.theme.Text.Muted))
	vm.queueSearch.hint.SetText(fmt.Sprintf("[%s]Filter queue (regex, case-insensitive). Enter to apply. Esc to cancel.[-]", vm.theme.Text.Muted))

	vm.queueSearch.input.SetChangedFunc(func(_ string) {
		if vm.queueSearch.hint != nil {
			vm.queueSearch.hint.SetText(fmt.Sprintf("[%s]Filter queue (regex, case-insensitive). Enter to apply. Esc to cancel.[-]", vm.theme.Text.Muted))
		}
	})

	searchContainer := vm.newSearchContainer(vm.queueSearch.hint, vm.queueSearch.input)

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
	itemCount := len(vm.displayItems)
	if itemCount <= 0 {
		return
	}

	row, _ := vm.table.GetSelection()
	itemIdx := rowToItem(row)
	if itemIdx < 0 {
		itemIdx = 0
	}

	itemIdx += delta
	if itemIdx < 0 {
		itemIdx = 0
	}
	if itemIdx >= itemCount {
		itemIdx = itemCount - 1
	}

	vm.table.Select(itemToFirstRow(itemIdx), 0)
	vm.applySelectionStyling()
}

func (vm *viewModel) selectQueueTop() {
	if len(vm.displayItems) == 0 {
		return
	}
	vm.table.Select(itemToFirstRow(0), 0)
	vm.applySelectionStyling()
}

func (vm *viewModel) selectQueueBottom() {
	itemCount := len(vm.displayItems)
	if itemCount <= 0 {
		return
	}
	vm.table.Select(itemToFirstRow(itemCount-1), 0)
	vm.applySelectionStyling()
}
