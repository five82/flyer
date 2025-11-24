package ui

import (
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
		vm.detail.SetText("[#64748b]Select an item to view details[-]")
		return
	}
	item := vm.displayItems[row-1]
	var b strings.Builder

	writeSection := func(title string) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "[#cbd5e1::b]%s[-:-:-]\n", title) // Slate-300, bold
	}

	writeRow := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		fmt.Fprintf(&b, "[#94a3b8]%s[-] %s\n", padLabel(label), value) // Slate-400
	}

	formatPath := func(path string) string {
		path = strings.TrimSpace(path)
		if path == "" {
			return ""
		}
		base := filepath.Base(path)
		if base == "." || base == "/" || base == "" {
			base = path
		}
		return fmt.Sprintf("[#38bdf8]%s[-]", tview.Escape(base)) // Sky-400
	}

	formatLogPath := func(path, missing string) string {
		path = strings.TrimSpace(path)
		if path == "" {
			return fmt.Sprintf("[#475569]%s[-]", missing) // Slate-600
		}
		return fmt.Sprintf("[#38bdf8]%s[-]", tview.Escape(truncateMiddle(path, 72)))
	}

	// Title and chips
	writeSection("Summary")
	title := composeTitle(item)
	titleValue := fmt.Sprintf("[#f8fafc]%s[-]", tview.Escape(title)) // Slate-50
	if item.NeedsReview {
		titleValue += " " + badge("REVIEW", "#fbbf24")
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		titleValue += " " + badge("ERROR", "#f87171")
	}
	writeRow("Title", titleValue)

	status := statusChip(item.Status)
	lane := laneChip(determineLane(item.Status))
	percent := clampPercent(item.Progress.Percent)
	statusLine := fmt.Sprintf("%s  %s  [#64748b]ID %d[-]  [#94a3b8]%3.0f%%[-]", status, lane, item.ID, percent)
	writeRow("Status", statusLine)

	// Progress block
	writeSection("Progress")
	stage := strings.TrimSpace(item.Progress.Stage)
	if stage == "" {
		stage = titleCase(item.Status)
	}
	writeRow("Stage", fmt.Sprintf("[#38bdf8]%s[-]", tview.Escape(stage)))

	progress := detailProgressBar(item)
	stageMsg := strings.TrimSpace(item.Progress.Message)
	writeRow("Progress", progress)
	if stageMsg != "" {
		writeRow("Note", fmt.Sprintf("[#94a3b8]%s[-]", tview.Escape(stageMsg)))
	}

	// Issues block
	if strings.TrimSpace(item.ErrorMessage) != "" {
		writeSection("Issues")
		writeRow("Error", fmt.Sprintf("[#f87171]%s[-]", tview.Escape(item.ErrorMessage)))
	}
	if item.NeedsReview {
		if strings.TrimSpace(item.ErrorMessage) == "" {
			writeSection("Issues")
		}
		reason := strings.TrimSpace(item.ReviewReason)
		if reason == "" {
			reason = "Needs operator review"
		}
		writeRow("Review", fmt.Sprintf("[#fbbf24]%s[-]", tview.Escape(reason)))
	}

	// Artifacts
	writeSection("Artifacts")
	writeRow("Source", formatPath(item.SourcePath))
	writeRow("Rip", formatPath(item.RippedFile))
	writeRow("Encoded", formatPath(item.EncodedFile))
	writeRow("Final", formatPath(item.FinalFile))

	// Logs
	writeSection("Logs")
	writeRow("Daemon", formatLogPath(vm.options.DaemonLogPath, "not configured"))
	writeRow("Encoding", formatLogPath(vm.options.DraptoLogPath, "not configured"))
	writeRow("Item", formatLogPath(item.BackgroundLogPath, "not available for this item"))

	// Metadata
	if metaRows := summarizeMetadata(item.Metadata); len(metaRows) > 0 {
		writeSection("Metadata")
		b.WriteString(formatMetadata(metaRows))
	}

	// Mini timeline
	created := item.ParsedCreatedAt()
	if !created.IsZero() {
		writeSection("Timeline")
		fmt.Fprintf(&b, "  [#94a3b8]Created[-] [#38bdf8]%s[-] [#64748b](%s ago)[-]\n", formatLocalTimestamp(created), humanizeDuration(time.Since(created)))
	}

	// Rip spec summary
	if summary, err := item.ParseRipSpec(); err == nil {
		const maxTitles = 6
		if summary.ContentKey != "" || len(summary.Titles) > 0 {
			writeSection("Rip Spec")
		}
		if summary.ContentKey != "" {
			fmt.Fprintf(&b, "  [#94a3b8]Key[-]   [#38bdf8]%s[-]\n", summary.ContentKey)
		}
		count := len(summary.Titles)
		for i, title := range summary.Titles {
			if i >= maxTitles {
				fmt.Fprintf(&b, "  [#94a3b8]…[-] [#64748b]+%d more titles[-]\n", count-maxTitles)
				break
			}
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
			fmt.Fprintf(&b, "  [#38bdf8]- %s[-] [#818cf8]%02d:%02d[-]", tview.Escape(name), minutes, seconds)
			if fingerprint != "" {
				fmt.Fprintf(&b, " [#94a3b8]%s[-]", fingerprint)
			}
			b.WriteString("\n")
		}
	}

	vm.detail.SetText(b.String())
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

func formatMetadata(rows []metadataRow) string {
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
		fmt.Fprintf(&b, "  [#94a3b8]%s[-] [#38bdf8]%s[-]\n", key, values[i])
	}
	return b.String()
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

func detailProgressBar(item spindle.QueueItem) string {
	percent := clampPercent(item.Progress.Percent)
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
		vm.logView.SetTitle(" [::b]Item Log[::-] ")
	case logSourceEncoding:
		vm.logView.SetTitle(" [::b]Encoding Log[::-] ")
	default:
		vm.logView.SetTitle(" [::b]Daemon Log[::-] ")
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
