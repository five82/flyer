package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
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
	vm.resetLogBuffer()
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
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
			vm.showQueueView()
		}
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
	vm.renderTable()
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

func formatLocalTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("Mon Jan 2 2006 15:04")
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

func (vm *viewModel) describeItemFileSummary(item spindle.QueueItem, stage string, expanded bool) string {
	current := normalizeEpisodeStage(stage)
	entries := []struct {
		label  string
		path   string
		expect string
	}{
		{"Src", item.SourcePath, ""},
		{"Rip", item.RippedFile, "ripping"},
		{"Enc", item.EncodedFile, "encoding"},
		{"Fin", item.FinalFile, "final"},
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		path := strings.TrimSpace(e.path)
		symbol := "–"
		if path != "" {
			symbol = "✓"
		} else if e.expect != "" && current == e.expect {
			symbol = "…"
		}
		name := "pending"
		if symbol == "…" {
			name = "in flight"
		}
		if path != "" {
			if expanded {
				name = truncateMiddle(path, 72)
			} else {
				base := filepath.Base(path)
				if base == "" || base == "." || base == "/" {
					base = truncateMiddle(path, 28)
				}
				name = truncate(base, 28)
			}
		}
		parts = append(parts, fmt.Sprintf("[%s]%s %s[-] [%s]%s[-]", vm.theme.Text.Muted, e.label, symbol, vm.theme.Text.Accent, tview.Escape(name)))
	}
	return strings.Join(parts, "  ")
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
	if !force && time.Since(vm.lastLogSet) < 400*time.Millisecond {
		return
	}

	if vm.logMode == logSourceItem {
		vm.refreshItemLogs()
		return
	}

	vm.refreshStreamLogs()
}

func (vm *viewModel) refreshItemLogs() {
	item := vm.selectedItem()
	if item == nil {
		vm.logView.SetText("Select an item to view logs")
		vm.rawLogLines = nil
		vm.updateLogStatus(false, "")
		return
	}
	path := strings.TrimSpace(item.BackgroundLogPath)
	if path == "" {
		vm.logView.SetText("No background log for this item")
		vm.rawLogLines = nil
		vm.updateLogStatus(false, "")
		return
	}

	lines, err := logtail.Read(path, maxLogLines)
	if err != nil {
		vm.logView.SetText(fmt.Sprintf("Error reading background log: %v", err))
		vm.updateLogStatus(false, path)
		return
	}
	if len(lines) == 0 {
		vm.logView.SetText("No background log entries available")
		vm.rawLogLines = nil
		vm.updateLogStatus(false, path)
		return
	}

	vm.rawLogLines = append([]string(nil), lines...)
	colorized := logtail.ColorizeLines(lines)
	vm.displayLog(colorized, path)
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

	req := spindle.LogQuery{
		Since: vm.logCursor[key],
		Limit: maxLogLines,
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
		if len(vm.rawLogLines) > maxLogLines {
			vm.rawLogLines = vm.rawLogLines[len(vm.rawLogLines)-maxLogLines:]
		}
	}
	if len(vm.rawLogLines) == 0 {
		vm.logView.SetText("No log entries available")
		vm.updateLogStatus(false, "api stream")
		return
	}
	colorized := logtail.ColorizeLines(vm.rawLogLines)
	vm.displayLog(colorized, "api stream")
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
		vm.logView.SetTitle(" [::b]Item Log[::-] ")
	default:
		vm.logView.SetTitle(" [::b]Daemon Log[::-] ")
	}
}

func (vm *viewModel) resetLogBuffer() {
	vm.rawLogLines = nil
	vm.lastLogPath = ""
	vm.lastLogKey = ""
	vm.logCursor = make(map[string]uint64)
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

func (vm *viewModel) currentItemID() int64 {
	if vm.logMode != logSourceItem {
		return 0
	}
	item := vm.selectedItem()
	if item == nil {
		vm.currentItemLogID = 0
		return 0
	}
	vm.currentItemLogID = item.ID
	return item.ID
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
	if path != "" {
		status += fmt.Sprintf(" • [%s]%s[-]", vm.theme.Text.AccentSoft, truncate(path, 40))
	}
	vm.searchStatus.SetText(status)
}
