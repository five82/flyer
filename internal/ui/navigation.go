package ui

import (
	"github.com/five82/flyer/internal/spindle"
)

func (vm *viewModel) showDetailView() {
	// In dual-pane mode there is always a detail pane; treat this as focusing it.
	vm.currentView = "queue"
	vm.mainContent.SwitchToPage("queue")
	vm.clearSearch()
	row, _ := vm.table.GetSelection()
	vm.updateDetail(row)
	vm.focusDetailPane()
}

func (vm *viewModel) showQueueView() {
	vm.currentView = "queue"
	vm.mainContent.SwitchToPage("queue")
	vm.clearSearch()
	vm.focusQueuePane()
}

func (vm *viewModel) showLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	vm.updateLogTitle()
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
	vm.setCommandBar("logs")
}

func (vm *viewModel) showProblemsView() {
	vm.currentView = "problems"
	vm.mainContent.SwitchToPage("problems")
	vm.clearSearch()
	vm.refreshProblems(true)
	vm.app.SetFocus(vm.problemsView)
	vm.setCommandBar("problems")
}

func (vm *viewModel) showItemLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	// Force item log mode
	vm.logs.mode = logSourceItem
	vm.updateLogTitle()
	vm.resetLogBuffer()
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
	vm.setCommandBar("logs")
}

func (vm *viewModel) showDaemonLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	// Force daemon log mode
	vm.logs.mode = logSourceDaemon
	vm.updateLogTitle()
	vm.resetLogBuffer()
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
	vm.setCommandBar("logs")
}

func (vm *viewModel) focusQueuePane() {
	vm.app.SetFocus(vm.table)
	vm.setCommandBar("queue")
}

func (vm *viewModel) focusDetailPane() {
	vm.app.SetFocus(vm.detail)
	vm.setCommandBar("detail")
}

func (vm *viewModel) currentCommandView() string {
	switch vm.app.GetFocus() {
	case vm.detail:
		return "detail"
	case vm.logView:
		return "logs"
	case vm.problemsView:
		return "problems"
	default:
		return "queue"
	}
}

func (vm *viewModel) toggleFocus() {
	focus := vm.app.GetFocus()
	switch focus {
	case vm.table:
		vm.focusDetailPane()
	case vm.detail:
		vm.showItemLogsView()
	case vm.logView:
		vm.showProblemsView()
	case vm.problemsView:
		vm.showQueueView()
	default:
		vm.focusQueuePane()
	}
}

func (vm *viewModel) toggleFocusReverse() {
	focus := vm.app.GetFocus()
	switch focus {
	case vm.table:
		vm.showProblemsView()
	case vm.detail:
		vm.focusQueuePane()
	case vm.logView:
		vm.showDetailView()
	case vm.problemsView:
		vm.showItemLogsView()
	default:
		vm.focusQueuePane()
	}
}

func (vm *viewModel) cycleFilter() {
	switch vm.filterMode {
	case filterAll:
		vm.filterMode = filterFailed
	case filterFailed:
		vm.filterMode = filterReview
	case filterReview:
		vm.filterMode = filterProcessing
	default:
		vm.filterMode = filterAll
	}
	vm.renderTablePreservingSelection()
	vm.ensureSelection()
	vm.setCommandBar(vm.currentView)
}

func (vm *viewModel) episodesCollapsed(itemID int64) bool {
	value, ok := vm.detailState.episodeCollapsed[itemID]
	if !ok {
		return true
	}
	return value
}

func (vm *viewModel) pathsExpanded(itemID int64) bool {
	return vm.detailState.pathExpanded[itemID]
}

func (vm *viewModel) toggleEpisodesCollapsed() {
	item := vm.selectedItem()
	if item == nil {
		return
	}
	vm.detailState.episodeCollapsed[item.ID] = !vm.episodesCollapsed(item.ID)
	row, _ := vm.table.GetSelection()
	vm.updateDetail(row)
	vm.setCommandBar(vm.currentCommandView())
}

func (vm *viewModel) togglePathDetail() {
	item := vm.selectedItem()
	if item == nil {
		return
	}
	vm.detailState.pathExpanded[item.ID] = !vm.pathsExpanded(item.ID)
	row, _ := vm.table.GetSelection()
	vm.updateDetail(row)
	vm.setCommandBar(vm.currentCommandView())
}

func (vm *viewModel) toggleFullscreen() {
	vm.detailState.fullscreenMode = !vm.detailState.fullscreenMode
	focus := vm.app.GetFocus()

	if vm.detailState.fullscreenMode {
		// Enter fullscreen mode
		switch focus {
		case vm.detail:
			vm.mainContent.SwitchToPage("detail-fullscreen")
			vm.app.SetFocus(vm.detail)
		default:
			// Log view is already fullscreen in its own page
		}
	} else {
		// Exit fullscreen mode
		vm.mainContent.SwitchToPage("queue")
		row, _ := vm.table.GetSelection()
		vm.updateDetail(row)
		if focus == vm.detail {
			vm.app.SetFocus(vm.detail)
		}
	}

	vm.setCommandBar(vm.currentCommandView())
}

func (vm *viewModel) returnToCurrentView() {
	switch vm.currentView {
	case "queue":
		vm.app.SetFocus(vm.table)
	case "detail":
		vm.app.SetFocus(vm.detail)
	case "logs":
		vm.app.SetFocus(vm.logView)
	}
}

func clampPercent(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

func (vm *viewModel) selectedItem() *spindle.QueueItem {
	row, _ := vm.table.GetSelection()
	itemIdx := rowToItem(row)
	if itemIdx < 0 || itemIdx >= len(vm.displayItems) {
		return nil
	}
	item := vm.displayItems[itemIdx]
	return &item
}
