package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/logtail"
	"github.com/five82/flyer/internal/spindle"
)

func (vm *viewModel) refreshLogs(force bool) {
	if vm.logs.mode == logSourceDaemon && vm.client == nil {
		vm.logView.SetText("Spindle daemon unavailable")
		return
	}
	if vm.search.mode || vm.search.regex != nil {
		vm.updateLogStatus(false, vm.logs.lastPath)
		return
	}
	if !force && !vm.logs.follow {
		vm.updateLogStatus(false, vm.logs.lastPath)
		return
	}
	if !force && time.Since(vm.logs.lastSet) < LogRefreshDebounce {
		return
	}

	if vm.logs.mode == logSourceItem {
		vm.refreshItemTailLogs()
		return
	}

	vm.refreshStreamLogs()
}

func (vm *viewModel) refreshItemTailLogs() {
	item := vm.selectedItem()
	if item == nil {
		vm.logView.SetText("Select an item to view logs")
		vm.logs.rawLines = nil
		vm.updateLogStatus(false, "")
		return
	}
	if vm.client == nil {
		vm.logView.SetText("Spindle daemon unavailable")
		vm.logs.rawLines = nil
		vm.updateLogStatus(false, "")
		return
	}

	key := fmt.Sprintf("item-%d", item.ID)
	offset := vm.logs.fileCursor[key]
	if key != vm.logs.lastFileKey {
		vm.logs.rawLines = nil
		offset = -1
		vm.logs.lastFileKey = key
	}

	ctx := vm.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, LogFetchTimeout)
	defer cancel()

	batch, err := vm.client.FetchLogTail(reqCtx, spindle.LogTailQuery{
		ItemID: item.ID,
		Offset: offset,
		Limit:  LogFetchLimit,
	})
	if err != nil {
		vm.logView.SetText(fmt.Sprintf("Error fetching item log: %v", err))
		vm.updateLogStatus(false, fmt.Sprintf("api tail item #%d", item.ID))
		return
	}

	vm.logs.fileCursor[key] = batch.Offset
	if len(batch.Lines) == 0 && len(vm.logs.rawLines) == 0 {
		vm.logView.SetText("No item log entries available")
		vm.updateLogStatus(false, fmt.Sprintf("api tail item #%d", item.ID))
		return
	}
	if len(batch.Lines) > 0 {
		vm.logs.rawLines = append(vm.logs.rawLines, batch.Lines...)
		if overflow := len(vm.logs.rawLines) - LogBufferLimit; overflow > 0 {
			vm.logs.rawLines = append([]string(nil), vm.logs.rawLines[overflow:]...)
		}
	}

	colorized := logtail.ColorizeLines(vm.logs.rawLines)
	vm.displayLog(colorized, fmt.Sprintf("api tail item #%d", item.ID))
	vm.logs.lastSet = time.Now()
}

func (vm *viewModel) refreshStreamLogs() {
	key := vm.streamLogKey()
	if key == "" {
		vm.logView.SetText("No log source available")
		vm.updateLogStatus(false, "")
		vm.logs.rawLines = nil
		vm.logs.lastKey = ""
		return
	}

	since := vm.logs.cursor[key]
	req := spindle.LogQuery{
		Since:     since,
		Limit:     LogFetchLimit,
		Component: vm.logs.filterComponent,
		Lane:      vm.logs.filterLane,
		Request:   vm.logs.filterRequest,
	}
	if since == 0 || key != vm.logs.lastKey {
		req.Tail = true
	}
	ctx := vm.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, LogFetchTimeout)
	defer cancel()

	batch, err := vm.client.FetchLogs(reqCtx, req)
	if err != nil {
		vm.logView.SetText(fmt.Sprintf("Error fetching logs: %v", err))
		vm.updateLogStatus(false, "api stream")
		return
	}

	if key != vm.logs.lastKey || req.Since == 0 {
		vm.logs.rawLines = nil
	}
	vm.logs.lastKey = key
	vm.logs.cursor[key] = batch.Next

	newLines := formatLogEvents(batch.Events)
	if len(newLines) > 0 {
		vm.logs.rawLines = append(vm.logs.rawLines, newLines...)
		if overflow := len(vm.logs.rawLines) - LogBufferLimit; overflow > 0 {
			vm.logs.rawLines = append([]string(nil), vm.logs.rawLines[overflow:]...)
		}
	}
	if len(vm.logs.rawLines) == 0 {
		vm.logView.SetText("No log entries available")
		vm.updateLogStatus(false, "api stream")
		return
	}
	colorized := logtail.ColorizeLines(vm.logs.rawLines)
	vm.displayLog(colorized, "api logs")
	vm.logs.lastSet = time.Now()
}

func (vm *viewModel) displayLog(colorizedLines []string, path string) {
	// Add line numbers to each line
	numberedLines := make([]string, len(colorizedLines))
	for i, line := range colorizedLines {
		lineNum := i + 1
		numberedLines[i] = fmt.Sprintf("[%s]%4d │[-] %s", vm.theme.Text.Faint, lineNum, line)
	}
	vm.logView.SetText(strings.Join(numberedLines, "\n"))
	if vm.logs.follow {
		vm.logView.ScrollToEnd()
	}
	vm.updateLogStatus(vm.logs.follow, path)
}

func (vm *viewModel) updateLogTitle() {
	switch vm.logs.mode {
	case logSourceItem:
		if item := vm.selectedItem(); item != nil && item.ID > 0 {
			vm.logView.SetTitle(fmt.Sprintf(" [::b]Item #%d Log[::-] ", item.ID))
			return
		}
		vm.logView.SetTitle(" [::b]Item Log[::-] ")
	default:
		title := " [::b]Daemon Log[::-] "
		if vm.logFiltersActive() {
			title = fmt.Sprintf(" [::b]Daemon Log[::-] [%s]filtered[-] ", vm.theme.Text.Warning)
		}
		vm.logView.SetTitle(title)
	}
}

func (vm *viewModel) resetLogBuffer() {
	vm.logs.rawLines = nil
	vm.logs.lastPath = ""
	vm.logs.lastKey = ""
	vm.logs.lastFileKey = ""
	vm.logs.cursor = make(map[string]uint64)
	vm.logs.fileCursor = make(map[string]int64)
}

func (vm *viewModel) streamLogKey() string {
	switch vm.logs.mode {
	case logSourceDaemon:
		return "daemon"
	case logSourceItem:
		item := vm.selectedItem()
		if item == nil {
			vm.logs.currentItemID = 0
			return ""
		}
		if vm.logs.currentItemID != item.ID {
			vm.logs.currentItemID = item.ID
		}
		return fmt.Sprintf("item-%d", item.ID)
	default:
		return ""
	}
}

// updateLogStatus refreshes the footer line without clobbering active search info.
func (vm *viewModel) updateLogStatus(active bool, path string) {
	vm.logs.lastPath = path
	if vm.search.mode || vm.search.regex != nil {
		// search status owns the bar
		return
	}
	var src string
	switch vm.logs.mode {
	case logSourceItem:
		src = "Item"
	default:
		src = "Daemon"
	}
	lineCount := len(vm.logs.rawLines)
	status := fmt.Sprintf("[%s]%s log[-] [%s]%d lines[-] [%s]auto-tail %s[-]",
		vm.theme.Text.Faint,
		src,
		vm.theme.Text.Muted,
		lineCount,
		vm.theme.Text.Faint,
		ternary(active, "on", "off"))

	if vm.logs.mode == logSourceDaemon && vm.logFiltersActive() {
		filters := []string{}
		if component := strings.TrimSpace(vm.logs.filterComponent); component != "" {
			filters = append(filters, "comp="+component)
		}
		if lane := strings.TrimSpace(vm.logs.filterLane); lane != "" {
			filters = append(filters, "lane="+lane)
		}
		if req := strings.TrimSpace(vm.logs.filterRequest); req != "" {
			filters = append(filters, "req="+truncateMiddle(req, 24))
		}
		if len(filters) > 0 {
			status += fmt.Sprintf(" • [%s]filter[-] [%s]%s[-]",
				vm.theme.Text.Faint,
				vm.theme.Text.Secondary,
				tview.Escape(truncate(strings.Join(filters, " "), 48)))
		}
	}
	if path != "" {
		status += fmt.Sprintf(" • [%s]%s[-]", vm.theme.Text.AccentSoft, truncate(path, 40))
	}
	vm.search.status.SetText(status)
}
