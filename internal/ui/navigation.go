package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/five82/flyer/internal/logtail"
	"github.com/five82/flyer/internal/spindle"
)

func (vm *viewModel) showDetailView() {
	vm.currentView = "detail"
	vm.mainContent.SwitchToPage("detail")
	vm.clearSearch()
	row, _ := vm.table.GetSelection()
	vm.updateDetail(row)
	vm.app.SetFocus(vm.detail)
	vm.setCommandBar("detail")
}

func (vm *viewModel) showQueueView() {
	vm.currentView = "queue"
	vm.mainContent.SwitchToPage("queue")
	vm.clearSearch()
	vm.app.SetFocus(vm.table)
	vm.setCommandBar("queue")
}

func (vm *viewModel) showLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	vm.updateLogTitle()
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
	vm.setCommandBar("logs")
}

func (vm *viewModel) showItemLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	// Force item log mode
	vm.logMode = logSourceItem
	vm.updateLogTitle()
	vm.lastLogPath = ""
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
}

func (vm *viewModel) showDaemonLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	// Force daemon log mode
	vm.logMode = logSourceDaemon
	vm.updateLogTitle()
	vm.lastLogPath = ""
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
}

func (vm *viewModel) showEncodingLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	// Force encoding log mode
	vm.logMode = logSourceEncoding
	vm.updateLogTitle()
	vm.lastLogPath = ""
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
}

func (vm *viewModel) toggleFocus() {
	switch vm.currentView {
	case "queue":
		vm.showDetailView()
	case "detail":
		vm.showDaemonLogsView()
	case "logs":
		switch vm.logMode {
		case logSourceDaemon:
			vm.showEncodingLogsView()
		case logSourceEncoding:
			vm.showItemLogsView()
		default:
			vm.showQueueView()
		}
	}
}

func (vm *viewModel) toggleLogSource() {
	switch vm.logMode {
	case logSourceDaemon:
		vm.logMode = logSourceEncoding
	case logSourceEncoding:
		vm.logMode = logSourceItem
	default:
		vm.logMode = logSourceDaemon
	}
	vm.updateLogTitle()
	vm.lastLogPath = ""
	// Always show logs view when toggling log source
	vm.showLogsView()
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
	vm.renderTable()
	vm.ensureSelection()
	vm.setCommandBar(vm.currentView)
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

func (vm *viewModel) updateDetail(row int) {
	if row <= 0 || row-1 >= len(vm.displayItems) {
		vm.detail.SetText("[cadetblue]Select an item to view details[-]")
		return
	}
	item := vm.displayItems[row-1]
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("[fuchsia]Title:[-] [dodgerblue]%s[-]\n", composeTitle(item)))
	builder.WriteString(fmt.Sprintf("[fuchsia]Status:[-] [dodgerblue]%s[-]\n", strings.ToUpper(item.Status)))
	builder.WriteString(fmt.Sprintf("[fuchsia]Lane:[-] [dodgerblue]%s[-]\n", determineLane(item.Status)))
	if item.Progress.Stage != "" || item.Progress.Message != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Stage:[-] [dodgerblue]%s[-] ([cadetblue]%s[-] [dodgerblue]%.0f%%[-])\n", strings.TrimSpace(item.Progress.Stage), strings.TrimSpace(item.Progress.Message), item.Progress.Percent))
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		builder.WriteString(fmt.Sprintf("[orangered]Error:[-] [red]%s[-]\n", item.ErrorMessage))
	}
	if item.NeedsReview {
		builder.WriteString("[darkorange]Needs review[-]\n")
	}
	if item.ReviewReason != "" {
		builder.WriteString(fmt.Sprintf("[darkorange]Reason:[-] [dodgerblue]%s[-]\n", item.ReviewReason))
	}
	if item.RippedFile != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Rip:[-] [cadetblue]%s[-]\n", item.RippedFile))
	}
	if item.EncodedFile != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Encoded:[-] [cadetblue]%s[-]\n", item.EncodedFile))
	}
	if item.FinalFile != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Final:[-] [cadetblue]%s[-]\n", item.FinalFile))
	}
	if item.BackgroundLogPath != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Background log:[-] [cadetblue]%s[-]\n", item.BackgroundLogPath))
	}
	if summary, err := item.ParseRipSpec(); err == nil {
		if summary.ContentKey != "" {
			builder.WriteString(fmt.Sprintf("[fuchsia]Content Key:[-] [cadetblue]%s[-]\n", summary.ContentKey))
		}
		if len(summary.Titles) > 0 {
			builder.WriteString("[fuchsia]Titles:[-]\n")
			for _, title := range summary.Titles {
				name := strings.TrimSpace(title.Name)
				if name == "" {
					name = fmt.Sprintf("Title %d", title.ID)
				}
				fingerprint := strings.TrimSpace(title.ContentFingerprint)
				if len(fingerprint) > 16 {
					fingerprint = fingerprint[:16]
				}
				minutes := title.Duration / 60
				seconds := title.Duration % 60
				builder.WriteString(fmt.Sprintf("  [cadetblue]- %s[-] [dodgerblue]%02d:%02d[-]", name, minutes, seconds))
				if fingerprint != "" {
					builder.WriteString(fmt.Sprintf(" [lightskyblue]%s[-]", fingerprint))
				}
				builder.WriteString("\n")
			}
		}
	}
	if ts := item.ParsedCreatedAt(); !ts.IsZero() {
		builder.WriteString(fmt.Sprintf("[fuchsia]Created:[-] [cadetblue]%s[-]\n", ts.Format(time.RFC3339)))
	}
	if ts := item.ParsedUpdatedAt(); !ts.IsZero() {
		builder.WriteString(fmt.Sprintf("[fuchsia]Updated:[-] [cadetblue]%s[-]\n", ts.Format(time.RFC3339)))
	}
	vm.detail.SetText(builder.String())
}

func (vm *viewModel) refreshLogs(force bool) {
	var path string
	switch vm.logMode {
	case logSourceDaemon:
		path = vm.options.DaemonLogPath
	case logSourceEncoding:
		path = vm.options.DraptoLogPath
	case logSourceItem:
		item := vm.selectedItem()
		if item == nil || strings.TrimSpace(item.BackgroundLogPath) == "" {
			vm.logView.SetText("No background log for this item")
			vm.lastLogPath = ""
			return
		}
		path = item.BackgroundLogPath
	}
	if path == "" {
		switch vm.logMode {
		case logSourceEncoding:
			vm.logView.SetText("Encoding log path not configured")
		case logSourceItem:
			vm.logView.SetText("No background log for this item")
		default:
			vm.logView.SetText("Log path not configured")
		}
		vm.updateLogStatus(false, path)
		return
	}
	if !force && (vm.searchMode || vm.searchRegex != nil) {
		vm.updateLogStatus(false, path)
		return
	}
	if !force && path == vm.lastLogPath && time.Since(vm.lastLogSet) < 500*time.Millisecond {
		return
	}
	lines, err := logtail.Read(path, maxLogLines)
	if err != nil {
		vm.logView.SetText(fmt.Sprintf("Error reading log: %v", err))
		vm.lastLogPath = path
		vm.lastLogSet = time.Now()
		vm.updateLogStatus(false, path)
		return
	}
	vm.rawLogLines = lines
	colorizedLines := logtail.ColorizeLines(lines)
	vm.displayLog(colorizedLines, path)
	vm.lastLogPath = path
	vm.lastLogSet = time.Now()
}

func (vm *viewModel) displayLog(colorizedLines []string, path string) {
	vm.logView.SetText(strings.Join(colorizedLines, "\n"))
	vm.updateLogStatus(true, path)
}

func (vm *viewModel) selectedItem() *spindle.QueueItem {
	row, _ := vm.table.GetSelection()
	if row <= 0 || row-1 >= len(vm.displayItems) {
		return nil
	}
	item := vm.displayItems[row-1]
	return &item
}

func (vm *viewModel) updateLogTitle() {
	switch vm.logMode {
	case logSourceItem:
		vm.logView.SetTitle(" [lightskyblue]Item Log[-] ")
	case logSourceEncoding:
		vm.logView.SetTitle(" [lightskyblue]Encoding Log[-] ")
	default:
		vm.logView.SetTitle(" [lightskyblue]Daemon Log[-] ")
	}
}

// updateLogStatus refreshes the footer line without clobbering active search info.
func (vm *viewModel) updateLogStatus(active bool, path string) {
	if vm.searchMode || vm.searchRegex != nil {
		// search status owns the bar
		return
	}
	var src string
	switch vm.logMode {
	case logSourceItem:
		src = "Item"
	case logSourceEncoding:
		src = "Encoding"
	default:
		src = "Daemon"
	}
	lineCount := len(vm.rawLogLines)
	status := fmt.Sprintf("[gray]%s log • %d lines • auto-tail %s[-]", src, lineCount, ternary(active, "on", "off"))
	if path != "" {
		status += fmt.Sprintf(" • %s", truncate(path, 40))
	}
	vm.searchStatus.SetText(status)
}
