package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/logtail"
	"github.com/five82/flyer/internal/spindle"
)

const detailLabelWidth = 9

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

func (vm *viewModel) focusQueuePane() {
	vm.app.SetFocus(vm.table)
	vm.setCommandBar("queue")
}

func (vm *viewModel) focusDetailPane() {
	vm.app.SetFocus(vm.detail)
	vm.setCommandBar("detail")
}

func (vm *viewModel) toggleFocus() {
	focus := vm.app.GetFocus()
	switch focus {
	case vm.table:
		vm.showDaemonLogsView()
	case vm.logView:
		switch vm.logMode {
		case logSourceDaemon:
			vm.showEncodingLogsView()
		case logSourceEncoding:
			vm.showItemLogsView()
		default:
			vm.showQueueView()
		}
	default:
		vm.focusQueuePane()
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
	var b strings.Builder

	writeRow := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		fmt.Fprintf(&b, "[slategray]%s[-] %s\n", padLabel(label), value)
	}

	// Title and chips
	title := composeTitle(item)
	writeRow("Title", fmt.Sprintf("[white]%s[-]", tview.Escape(title)))
	status := statusChip(item.Status)
	lane := laneChip(determineLane(item.Status))
	b.WriteString(fmt.Sprintf("[slategray]%s[-] %s  %s  [gray]ID %d[-]\n", padLabel("Status"), status, lane, item.ID))

	// Progress line with roomy bar and full stage message
	stage := strings.TrimSpace(item.Progress.Stage)
	if stage == "" {
		stage = titleCase(item.Status)
	}
	stageMsg := strings.TrimSpace(item.Progress.Message)
	progress := detailProgressBar(item)
	if stageMsg != "" {
		progress = fmt.Sprintf("%s  [#6c757d]%s[-]", progress, tview.Escape(truncate(stageMsg, 140)))
	}
	writeRow("Progress", progress)

	if strings.TrimSpace(item.ErrorMessage) != "" {
		writeRow("Error", fmt.Sprintf("[red]%s[-]", tview.Escape(item.ErrorMessage)))
	}
	if item.NeedsReview {
		reason := strings.TrimSpace(item.ReviewReason)
		if reason == "" {
			reason = "Needs operator review"
		}
		writeRow("Review", fmt.Sprintf("[darkorange]%s[-]", tview.Escape(reason)))
	}

	// Paths & artifacts (favor concise/meaningful pieces)
	ripName := strings.TrimSpace(item.RippedFile)
	if ripName != "" {
		ripName = filepath.Base(ripName)
		writeRow("Rip", fmt.Sprintf("[cadetblue]%s[-]", tview.Escape(ripName)))
	}
	if strings.TrimSpace(item.FinalFile) != "" {
		writeRow("Final", fmt.Sprintf("[cadetblue]%s[-]", tview.Escape(item.FinalFile)))
	}
	writeRow("Source", fmt.Sprintf("[cadetblue]%s[-]", tview.Escape(truncateMiddle(item.SourcePath, 64))))
	writeRow("Fingerprint", fmt.Sprintf("[lightskyblue]%s[-]", tview.Escape(truncate(item.DiscFingerprint, 48))))

	// Mini timeline
	created := item.ParsedCreatedAt()
	updated := item.ParsedUpdatedAt()
	if !created.IsZero() || !updated.IsZero() {
		b.WriteString("\n[slategray]Timeline[-]\n")
		if !created.IsZero() {
			fmt.Fprintf(&b, "  [slategray]Created[-] [cadetblue]%s[-] [#6c757d](%s ago)[-]\n", created.Format(time.RFC3339), humanizeDuration(time.Since(created)))
		}
		if !updated.IsZero() {
			fmt.Fprintf(&b, "  [slategray]Updated[-] [cadetblue]%s[-] [#6c757d](%s ago)[-]\n", updated.Format(time.RFC3339), humanizeDuration(time.Since(updated)))
		}
	}

	// Rip spec summary
	if summary, err := item.ParseRipSpec(); err == nil {
		if summary.ContentKey != "" || len(summary.Titles) > 0 {
			b.WriteString("\n[slategray]Rip Spec[-]\n")
		}
		if summary.ContentKey != "" {
			fmt.Fprintf(&b, "  [slategray]Key[-]   [cadetblue]%s[-]\n", summary.ContentKey)
		}
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
			fmt.Fprintf(&b, "  [cadetblue]- %s[-] [dodgerblue]%02d:%02d[-]", tview.Escape(name), minutes, seconds)
			if fingerprint != "" {
				fmt.Fprintf(&b, " [lightskyblue]%s[-]", fingerprint)
			}
			b.WriteString("\n")
		}
	}

	vm.detail.SetText(b.String())
}

func detailProgressBar(item spindle.QueueItem) string {
	percent := item.Progress.Percent
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	const barWidth = 24
	filled := int(percent/100*barWidth + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := "[" + strings.Repeat("=", filled) + strings.Repeat(".", barWidth-filled) + "]"
	return fmt.Sprintf("[%s]%s[-] %3.0f%%", colorForStatus(item.Status), bar, percent)
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

func padLabel(label string) string {
	return fmt.Sprintf("%-*s", detailLabelWidth, label+":")
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
