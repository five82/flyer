package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (vm *viewModel) logFiltersActive() bool {
	return strings.TrimSpace(vm.logs.filterComponent) != "" ||
		strings.TrimSpace(vm.logs.filterLane) != "" ||
		strings.TrimSpace(vm.logs.filterRequest) != ""
}

func (vm *viewModel) showLogFilters() {
	if vm == nil || vm.root == nil {
		return
	}

	if vm.currentView != "logs" {
		vm.showLogsView()
	}

	componentField := tview.NewInputField().
		SetLabel("Component: ").
		SetText(vm.logs.filterComponent).
		SetFieldWidth(28)
	componentField.SetBackgroundColor(vm.theme.SurfaceColor())
	componentField.SetFieldBackgroundColor(vm.theme.SurfaceAltColor())
	componentField.SetFieldTextColor(hexToColor(vm.theme.Text.Primary))
	componentField.SetLabelColor(hexToColor(vm.theme.Text.Muted))

	laneField := tview.NewInputField().
		SetLabel("Lane:      ").
		SetText(vm.logs.filterLane).
		SetFieldWidth(28)
	laneField.SetBackgroundColor(vm.theme.SurfaceColor())
	laneField.SetFieldBackgroundColor(vm.theme.SurfaceAltColor())
	laneField.SetFieldTextColor(hexToColor(vm.theme.Text.Primary))
	laneField.SetLabelColor(hexToColor(vm.theme.Text.Muted))

	requestField := tview.NewInputField().
		SetLabel("Request:   ").
		SetText(vm.logs.filterRequest).
		SetFieldWidth(28)
	requestField.SetBackgroundColor(vm.theme.SurfaceColor())
	requestField.SetFieldBackgroundColor(vm.theme.SurfaceAltColor())
	requestField.SetFieldTextColor(hexToColor(vm.theme.Text.Primary))
	requestField.SetLabelColor(hexToColor(vm.theme.Text.Muted))

	hint := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	hint.SetBackgroundColor(vm.theme.SurfaceColor())
	hint.SetTextColor(hexToColor(vm.theme.Text.Muted))
	hint.SetText(fmt.Sprintf("[%s]Blank fields disable filters. Apply switches to daemon stream logs (filters do not apply to item logs).[-]", vm.theme.Text.Muted))

	form := tview.NewForm()
	form.SetBackgroundColor(vm.theme.SurfaceColor())
	form.SetBorder(false)
	form.AddFormItem(componentField)
	form.AddFormItem(laneField)
	form.AddFormItem(requestField)

	closeModal := func() {
		vm.root.RemovePage("modal")
		vm.returnToCurrentView()
		vm.setCommandBar(vm.currentCommandView())
	}

	apply := func(component, lane, request string) {
		vm.logs.filterComponent = strings.TrimSpace(component)
		vm.logs.filterLane = strings.TrimSpace(lane)
		vm.logs.filterRequest = strings.TrimSpace(request)
		if vm.logs.mode != logSourceDaemon {
			vm.logs.mode = logSourceDaemon
			vm.updateLogTitle()
		}
		vm.resetLogBuffer()
		closeModal()
		vm.refreshLogs(true)
		vm.setCommandBar("logs")
	}

	form.AddButton("Apply", func() {
		apply(componentField.GetText(), laneField.GetText(), requestField.GetText())
	})
	form.AddButton("Clear", func() {
		componentField.SetText("")
		laneField.SetText("")
		requestField.SetText("")
		apply("", "", "")
	})
	form.AddButton("Cancel", closeModal)
	form.SetButtonsAlign(tview.AlignRight)

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	content.SetBackgroundColor(vm.theme.SurfaceColor())
	content.SetBorder(true).
		SetTitle(" [::b]Log Filters[::-] ").
		SetBorderColor(vm.theme.BorderFocusColor())
	content.AddItem(hint, 2, 0, false)
	content.AddItem(form, 0, 1, true)

	content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEsc,
			event.Key() == tcell.KeyCtrlC:
			closeModal()
			return nil
		}
		return event
	})

	vm.root.RemovePage("modal")
	modalW, modalH := vm.modalDimensions(76, 14)
	vm.root.AddPage("modal", center(modalW, modalH, content), true, true)
	vm.app.SetFocus(componentField)
}
