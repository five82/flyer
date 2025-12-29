package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/logtail"
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
	vm.logMode = logSourceItem
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
	vm.logMode = logSourceDaemon
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
		vm.showDaemonLogsView()
	case vm.logView:
		if vm.logMode == logSourceDaemon {
			vm.showItemLogsView()
		} else {
			vm.showProblemsView()
		}
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
		if vm.logMode == logSourceItem {
			vm.showDaemonLogsView()
		} else {
			vm.showDetailView()
		}
	case vm.problemsView:
		vm.showItemLogsView()
	default:
		vm.focusQueuePane()
	}
}

func (vm *viewModel) toggleLogSource() {
	if vm.logMode == logSourceDaemon {
		vm.logMode = logSourceItem
	} else {
		vm.logMode = logSourceDaemon
	}
	vm.updateLogTitle()
	vm.resetLogBuffer()
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
	vm.renderTablePreservingSelection()
	vm.ensureSelection()
	vm.setCommandBar(vm.currentView)
}

func (vm *viewModel) episodesCollapsed(itemID int64) bool {
	value, ok := vm.episodeCollapsed[itemID]
	if !ok {
		return true
	}
	return value
}

func (vm *viewModel) pathsExpanded(itemID int64) bool {
	return vm.pathExpanded[itemID]
}

func (vm *viewModel) toggleEpisodesCollapsed() {
	item := vm.selectedItem()
	if item == nil {
		return
	}
	vm.episodeCollapsed[item.ID] = !vm.episodesCollapsed(item.ID)
	row, _ := vm.table.GetSelection()
	vm.updateDetail(row)
	vm.setCommandBar(vm.currentCommandView())
}

func (vm *viewModel) togglePathDetail() {
	item := vm.selectedItem()
	if item == nil {
		return
	}
	vm.pathExpanded[item.ID] = !vm.pathsExpanded(item.ID)
	row, _ := vm.table.GetSelection()
	vm.updateDetail(row)
	vm.setCommandBar(vm.currentCommandView())
}

func (vm *viewModel) toggleFullscreen() {
	vm.fullscreenMode = !vm.fullscreenMode
	focus := vm.app.GetFocus()

	if vm.fullscreenMode {
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

type metadataRow struct {
	key   string
	value string
}

func summarizeMetadata(raw json.RawMessage) []metadataRow {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj) == 0 {
		return nil
	}
	mediaType := ""
	if mt, ok := obj["media_type"]; ok {
		if s, ok := mt.(string); ok {
			mediaType = strings.ToLower(strings.TrimSpace(s))
		}
	}
	skip := map[string]struct{}{
		"vote_average": {},
		"vote_count":   {},
		"overview":     {},
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		lk := strings.ToLower(strings.TrimSpace(k))
		if _, ignore := skip[lk]; ignore {
			continue
		}
		if mediaType == "movie" && lk == "movie" {
			continue
		}
		if mediaType == "tv" && lk == "tv" {
			continue
		}
		if mediaType == "movie" && strings.EqualFold(k, "season_number") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]metadataRow, 0, len(keys))
	for _, k := range keys {
		val := obj[k]
		switch v := val.(type) {
		case string:
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			rows = append(rows, metadataRow{key: k, value: v})
		case float64:
			rows = append(rows, metadataRow{key: k, value: strconv.FormatFloat(v, 'f', -1, 64)})
		case bool:
			rows = append(rows, metadataRow{key: k, value: strconv.FormatBool(v)})
		default:
			// skip nested/complex values to keep the view compact
		}
	}
	return rows
}

func (vm *viewModel) formatMetadata(rows []metadataRow) string {
	if len(rows) == 0 {
		return ""
	}
	ordered := reorderMetadata(rows)
	pretties := make([]string, len(ordered))
	values := make([]string, len(ordered))
	maxKey := 0
	for i, r := range ordered {
		key := prettifyMetaKey(r.key)
		pretties[i] = key
		if l := len([]rune(key)); l > maxKey {
			maxKey = l
		}
		val := strings.TrimSpace(r.value)
		if val == "" {
			val = "—"
		}
		val = truncate(val, 90)
		values[i] = tview.Escape(val)
	}
	if maxKey < 8 {
		maxKey = 8
	}
	if maxKey > 18 {
		maxKey = 18
	}
	var b strings.Builder
	for i := range ordered {
		key := padRight(truncate(pretties[i], maxKey), maxKey)
		fmt.Fprintf(&b, "  [%s]%s[-] [%s]%s[-]\n", vm.theme.Text.Muted, key, vm.theme.Text.Accent, values[i])
	}
	return b.String()
}

func detectMediaType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"media_type", "type"} {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok {
				return strings.ToLower(strings.TrimSpace(s))
			}
		}
	}
	return ""
}

func prettifyMetaKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, "_", " ")
	key = strings.ReplaceAll(key, ".", " ")
	parts := strings.Fields(key)
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}

func padRight(s string, width int) string {
	if width <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(r))
}

func reorderMetadata(rows []metadataRow) []metadataRow {
	if len(rows) == 0 {
		return rows
	}
	titleRows := make([]metadataRow, 0, 1)
	others := make([]metadataRow, 0, len(rows))
	for _, r := range rows {
		if strings.EqualFold(r.key, "title") {
			titleRows = append(titleRows, r)
		} else {
			others = append(others, r)
		}
	}
	return append(titleRows, others...)
}

func (vm *viewModel) refreshLogs(force bool) {
	if vm.logMode == logSourceDaemon && vm.client == nil {
		vm.logView.SetText("Spindle daemon unavailable")
		return
	}
	if vm.searchMode || vm.searchRegex != nil {
		vm.updateLogStatus(false, vm.lastLogPath)
		return
	}
	if !force && !vm.logFollow {
		vm.updateLogStatus(false, vm.lastLogPath)
		return
	}
	if !force && time.Since(vm.lastLogSet) < 400*time.Millisecond {
		return
	}

	if vm.logMode == logSourceItem {
		vm.refreshItemTailLogs()
		return
	}

	vm.refreshStreamLogs()
}

func (vm *viewModel) refreshItemTailLogs() {
	item := vm.selectedItem()
	if item == nil {
		vm.logView.SetText("Select an item to view logs")
		vm.rawLogLines = nil
		vm.updateLogStatus(false, "")
		return
	}
	if vm.client == nil {
		vm.logView.SetText("Spindle daemon unavailable")
		vm.rawLogLines = nil
		vm.updateLogStatus(false, "")
		return
	}

	key := fmt.Sprintf("item-%d", item.ID)
	offset := vm.logFileCursor[key]
	if key != vm.lastLogFileKey {
		vm.rawLogLines = nil
		offset = -1
		vm.lastLogFileKey = key
	}

	ctx := vm.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	batch, err := vm.client.FetchLogTail(reqCtx, spindle.LogTailQuery{
		ItemID: item.ID,
		Offset: offset,
		Limit:  logFetchLimit,
	})
	if err != nil {
		vm.logView.SetText(fmt.Sprintf("Error fetching item log: %v", err))
		vm.updateLogStatus(false, fmt.Sprintf("api tail item #%d", item.ID))
		return
	}

	vm.logFileCursor[key] = batch.Offset
	if len(batch.Lines) == 0 && len(vm.rawLogLines) == 0 {
		vm.logView.SetText("No background log entries available")
		vm.updateLogStatus(false, fmt.Sprintf("api tail item #%d", item.ID))
		return
	}
	if len(batch.Lines) > 0 {
		vm.rawLogLines = append(vm.rawLogLines, batch.Lines...)
		if overflow := len(vm.rawLogLines) - logBufferLimit; overflow > 0 {
			vm.rawLogLines = append([]string(nil), vm.rawLogLines[overflow:]...)
		}
	}

	colorized := logtail.ColorizeLines(vm.rawLogLines)
	vm.displayLog(colorized, fmt.Sprintf("api tail item #%d", item.ID))
	vm.lastLogSet = time.Now()
}

func (vm *viewModel) refreshStreamLogs() {
	key := vm.streamLogKey()
	if key == "" {
		vm.logView.SetText("No log source available")
		vm.updateLogStatus(false, "")
		vm.rawLogLines = nil
		vm.lastLogKey = ""
		return
	}

	since := vm.logCursor[key]
	req := spindle.LogQuery{
		Since:     since,
		Limit:     logFetchLimit,
		Component: vm.logFilterComponent,
		Lane:      vm.logFilterLane,
		Request:   vm.logFilterRequest,
	}
	if since == 0 || key != vm.lastLogKey {
		req.Tail = true
	}
	ctx := vm.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	batch, err := vm.client.FetchLogs(reqCtx, req)
	if err != nil {
		vm.logView.SetText(fmt.Sprintf("Error fetching logs: %v", err))
		vm.updateLogStatus(false, "api stream")
		return
	}

	if key != vm.lastLogKey || req.Since == 0 {
		vm.rawLogLines = nil
	}
	vm.lastLogKey = key
	vm.logCursor[key] = batch.Next

	newLines := formatLogEvents(batch.Events)
	if len(newLines) > 0 {
		vm.rawLogLines = append(vm.rawLogLines, newLines...)
		if overflow := len(vm.rawLogLines) - logBufferLimit; overflow > 0 {
			vm.rawLogLines = append([]string(nil), vm.rawLogLines[overflow:]...)
		}
	}
	if len(vm.rawLogLines) == 0 {
		vm.logView.SetText("No log entries available")
		vm.updateLogStatus(false, "api stream")
		return
	}
	colorized := logtail.ColorizeLines(vm.rawLogLines)
	vm.displayLog(colorized, "api logs")
	vm.lastLogSet = time.Now()
}

func (vm *viewModel) displayLog(colorizedLines []string, path string) {
	// Add line numbers to each line
	numberedLines := make([]string, len(colorizedLines))
	for i, line := range colorizedLines {
		lineNum := i + 1
		numberedLines[i] = fmt.Sprintf("[%s]%4d │[-] %s", vm.theme.Text.Faint, lineNum, line)
	}
	vm.logView.SetText(strings.Join(numberedLines, "\n"))
	if vm.logFollow {
		vm.logView.ScrollToEnd()
	}
	vm.updateLogStatus(vm.logFollow, path)
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
	vm.rawLogLines = nil
	vm.lastLogPath = ""
	vm.lastLogKey = ""
	vm.lastLogFileKey = ""
	vm.logCursor = make(map[string]uint64)
	vm.logFileCursor = make(map[string]int64)
}

func (vm *viewModel) streamLogKey() string {
	switch vm.logMode {
	case logSourceDaemon:
		return "daemon"
	case logSourceItem:
		item := vm.selectedItem()
		if item == nil {
			vm.currentItemLogID = 0
			return ""
		}
		if vm.currentItemLogID != item.ID {
			vm.currentItemLogID = item.ID
		}
		return fmt.Sprintf("item-%d", item.ID)
	default:
		return ""
	}
}

// updateLogStatus refreshes the footer line without clobbering active search info.
func (vm *viewModel) updateLogStatus(active bool, path string) {
	vm.lastLogPath = path
	if vm.searchMode || vm.searchRegex != nil {
		// search status owns the bar
		return
	}
	var src string
	switch vm.logMode {
	case logSourceItem:
		src = "Item"
	default:
		src = "Daemon"
	}
	lineCount := len(vm.rawLogLines)
	status := fmt.Sprintf("[%s]%s log[-] [%s]%d lines[-] [%s]auto-tail %s[-]",
		vm.theme.Text.Faint,
		src,
		vm.theme.Text.Muted,
		lineCount,
		vm.theme.Text.Faint,
		ternary(active, "on", "off"))

	if vm.logMode == logSourceDaemon && vm.logFiltersActive() {
		filters := []string{}
		if component := strings.TrimSpace(vm.logFilterComponent); component != "" {
			filters = append(filters, "comp="+component)
		}
		if lane := strings.TrimSpace(vm.logFilterLane); lane != "" {
			filters = append(filters, "lane="+lane)
		}
		if req := strings.TrimSpace(vm.logFilterRequest); req != "" {
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
	vm.searchStatus.SetText(status)
}
