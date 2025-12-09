package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

func (vm *viewModel) updateDetail(row int) {
	if row <= 0 || row-1 >= len(vm.displayItems) {
		vm.detail.SetText(fmt.Sprintf("[%s]Select an item to view details[-]", vm.theme.Text.Faint))
		vm.lastDetailID = 0
		return
	}
	item := vm.displayItems[row-1]
	summary, ripSpecErr := item.ParseRipSpec()
	titleLookup := make(map[int]*spindle.RipSpecTitleInfo)
	episodeTitleIndex := make(map[string]int)
	if ripSpecErr == nil {
		for _, title := range summary.Titles {
			t := title
			titleLookup[title.ID] = &t
		}
		for _, ep := range summary.Episodes {
			if ep.TitleID <= 0 {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(ep.Key))
			if key == "" {
				continue
			}
			episodeTitleIndex[key] = ep.TitleID
		}
	}
	episodes, totals := item.EpisodeSnapshot()
	activeEpisodeIndex := vm.activeEpisodeIndex(item, episodes)
	var focusEpisode *spindle.EpisodeStatus
	if activeEpisodeIndex >= 0 && activeEpisodeIndex < len(episodes) {
		focusEpisode = &episodes[activeEpisodeIndex]
	}
	currentStage := normalizeEpisodeStage(item.Progress.Stage)
	if currentStage == "" {
		currentStage = normalizeEpisodeStage(item.Status)
	}
	mediaType := detectMediaType(item.Metadata)
	var b strings.Builder
	text := vm.theme.Text

	writeSection := func(title string) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "[%s]%s[-]\n", text.Faint, strings.Repeat("─", 38))
		fmt.Fprintf(&b, "[%s::b]%s[-:-:-]\n", text.Secondary, title)
	}

	writeRow := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		fmt.Fprintf(&b, "[%s]%s[-] %s\n", text.Muted, padLabel(label), value)
	}

	formatLogRow := func(path, missing string) string {
		path = strings.TrimSpace(path)
		if path == "" {
			return fmt.Sprintf("[%s]%s[-]", text.Faint, missing)
		}
		display := fmt.Sprintf("[%s]%s[-]", text.Accent, tview.Escape(truncateMiddle(path, 64)))
		if preview := vm.logPreview(path); preview != "" {
			display = fmt.Sprintf("%s [%s]›[-] [%s]%s[-]", display, text.Faint, text.Muted, tview.Escape(preview))
		}
		return display
	}

	// Title and chips
	writeSection("Header")
	title := composeTitle(item)
	titleValue := fmt.Sprintf("[%s]%s[-]", text.Heading, tview.Escape(title))
	if item.NeedsReview {
		titleValue += " " + vm.badge("REVIEW", vm.theme.Badges.Review)
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		titleValue += " " + vm.badge("ERROR", vm.theme.Badges.Error)
	}
	writeRow("Title", titleValue)

	status := vm.statusChip(item.Status)
	lane := vm.laneChip(determineLane(item.Status))
	percent := clampPercent(item.Progress.Percent)
	statusLine := fmt.Sprintf("%s  %s  [%s]ID %d[-]  [%s]%3.0f%%[-]", status, lane, text.Faint, item.ID, text.Muted, percent)
	writeRow("Status", statusLine)

	// Chip strip (at-a-glance)
	chips := []string{}
	if epBadge := vm.episodeProgressBadge(item); epBadge != "" {
		chips = append(chips, epBadge)
	}
	if preset := item.DraptoPresetLabel(); preset != "" {
		chips = append(chips, vm.badge(strings.ToUpper(preset), vm.theme.StatusColor("pending")))
	}
	if metrics := formatEncodingMetrics(item.Encoding); metrics != "" {
		chips = append(chips, fmt.Sprintf("[%s]%s[-]", text.Secondary, tview.Escape(metrics)))
	}
	if len(chips) > 0 {
		writeRow("Chips", strings.Join(chips, "  "))
	}

	// Focus card
	if focusEpisode != nil {
		writeSection("Now")
		focusStage := vm.episodeStage(*focusEpisode, currentStage)
		writeRow("Episode", vm.formatEpisodeFocusLine(*focusEpisode, titleLookup, episodeTitleIndex, focusStage))
		if track := vm.describeEpisodeTrackInfo(focusEpisode, titleLookup, episodeTitleIndex); track != "" {
			writeRow("Track", track)
		}
		if files := vm.describeEpisodeFileStates(focusEpisode); files != "" {
			writeRow("Files", files)
		}
		if subs := vm.describeEpisodeSubtitleInfo(focusEpisode); subs != "" {
			writeRow("Subs", subs)
		}
	} else if mediaType == "movie" || len(episodes) == 0 {
		if cut := vm.movieFocusLine(summary, currentStage); cut != "" {
			writeSection("Now")
			writeRow("Cut", cut)
			if files := vm.describeItemFileStates(item, currentStage); files != "" {
				writeRow("Files", files)
			}
		}
	}

	// Progress block
	writeSection("Progress")
	stage := strings.TrimSpace(item.Progress.Stage)
	if stage == "" {
		stage = titleCase(item.Status)
	}
	writeRow("Stage", fmt.Sprintf("[%s]%s[-]", text.Accent, tview.Escape(stage)))
	writeRow("Flow", vm.detailProgressBar(item))
	if stageMsg := strings.TrimSpace(item.Progress.Message); stageMsg != "" {
		writeRow("Note", fmt.Sprintf("[%s]%s[-]", text.Muted, tview.Escape(stageMsg)))
	}

	// Issues block
	addNote := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		writeRow(label, value)
	}
	notesOpen := false
	openNotes := func() {
		if !notesOpen {
			writeSection("Notes")
			notesOpen = true
		}
	}
	if item.NeedsReview {
		openNotes()
		reason := strings.TrimSpace(item.ReviewReason)
		if reason == "" {
			reason = "Needs operator review"
		}
		addNote("Review", fmt.Sprintf("[%s]%s[-]", text.Warning, tview.Escape(reason)))
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		openNotes()
		addNote("Error", fmt.Sprintf("[%s]%s[-]", text.Danger, tview.Escape(item.ErrorMessage)))
	}

	// Artifacts
	writeSection("Files")
	filesExpanded := vm.pathsExpanded(item.ID)
	writeRow("Paths", vm.describeItemFileSummary(item, currentStage, filesExpanded))
	if !filesExpanded {
		writeRow("Hint", fmt.Sprintf("[%s]Press P to show full paths[-]", text.Faint))
	}

	if enc := item.Encoding; hasEncodingDetails(enc) {
		writeSection("Encoding")
		if hardware := formatEncodingHardware(enc); hardware != "" {
			writeRow("Hardware", fmt.Sprintf("[%s]%s[-]", text.Accent, tview.Escape(hardware)))
		}
		if cfg := formatEncodingConfig(enc); cfg != "" {
			writeRow("Config", fmt.Sprintf("[%s]%s[-]", text.Accent, tview.Escape(cfg)))
		}
		if source := formatEncodingSource(enc); source != "" {
			writeRow("Source", fmt.Sprintf("[%s]%s[-]", text.AccentSoft, tview.Escape(source)))
		}
		if crop := formatEncodingCrop(enc); crop != "" {
			writeRow("Crop", fmt.Sprintf("[%s]%s[-]", text.AccentSoft, tview.Escape(crop)))
		}
		if validation := formatEncodingValidation(enc); validation != "" {
			writeRow("Validation", fmt.Sprintf("[%s]%s[-]", text.AccentSoft, tview.Escape(validation)))
		}
		if result := formatEncodingResult(enc); result != "" {
			writeRow("Result", fmt.Sprintf("[%s]%s[-]", text.Accent, tview.Escape(result)))
		}
		if warning := strings.TrimSpace(enc.Warning); warning != "" {
			writeRow("Warning", fmt.Sprintf("[%s]%s[-]", text.Warning, tview.Escape(warning)))
		}
		if errSummary := formatEncodingIssue(enc.Error); errSummary != "" {
			writeRow("Encoder Error", fmt.Sprintf("[%s]%s[-]", text.Danger, tview.Escape(errSummary)))
		}
	}

	// Logs
	writeSection("Logs")
	writeRow("Daemon", formatLogRow(vm.options.DaemonLogPath, "not configured"))
	writeRow("Item", formatLogRow(item.BackgroundLogPath, "not available for this item"))

	// Metadata
	if metaRows := summarizeMetadata(item.Metadata); len(metaRows) > 0 {
		writeSection("Metadata")
		b.WriteString(vm.formatMetadata(metaRows))
	}

	if len(episodes) > 0 && mediaType != "movie" {
		writeSection("Episodes")
		if !item.EpisodesSynced {
			writeRow("Sync", fmt.Sprintf("[%s]Episode numbers not confirmed[-]", vm.theme.Text.Warning))
		}
		if summary := vm.describeEpisodeTotals(episodes, totals); summary != "" {
			fmt.Fprintf(&b, "  [%s]%s[-]\n", text.Muted, summary)
		}
		collapsed := vm.episodesCollapsed(item.ID)
		if collapsed {
			hidden := len(episodes)
			label := "episodes hidden"
			if hidden == 1 {
				label = "episode hidden"
			}
			fmt.Fprintf(&b, "  [%s]%d %s[-] [%s](press t to expand)[-]\n", text.Faint, hidden, label, text.Muted)
		} else {
			for idx, ep := range episodes {
				stage := vm.episodeStage(ep, currentStage)
				b.WriteString(vm.formatEpisodeLine(ep, titleLookup, episodeTitleIndex, idx == activeEpisodeIndex, stage))
			}
			fmt.Fprintf(&b, "  [%s]Press t to collapse[-]\n", text.Faint)
		}
	}

	// Mini timeline
	created := item.ParsedCreatedAt()
	if !created.IsZero() {
		writeSection("Timeline")
		fmt.Fprintf(&b, "  [%s]Created[-] [%s]%s[-] [%s](%s ago)[-]\n", text.Muted, text.Accent, formatLocalTimestamp(created), text.Faint, humanizeDuration(time.Since(created)))
	}

	// Rip spec summary
	if ripSpecErr == nil {
		const maxTitles = 6
		if summary.ContentKey != "" || len(summary.Titles) > 0 {
			writeSection("Rip Spec")
		}
		if summary.ContentKey != "" {
			fmt.Fprintf(&b, "  [%s]Key[-]   [%s]%s[-]\n", text.Muted, text.Accent, summary.ContentKey)
		}
		count := len(summary.Titles)
		for i, title := range summary.Titles {
			if i >= maxTitles {
				fmt.Fprintf(&b, "  [%s]…[-] [%s]+%d more titles[-]\n", text.Muted, text.Faint, count-maxTitles)
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
			fmt.Fprintf(&b, "  [%s]- %s[-] [%s]%02d:%02d[-]", text.Accent, tview.Escape(name), text.AccentSoft, minutes, seconds)
			if playlist := strings.TrimSpace(title.Playlist); playlist != "" {
				fmt.Fprintf(&b, " [%s]%s[-]", text.Warning, playlist)
			}
			if fingerprint != "" {
				fmt.Fprintf(&b, " [%s]%s[-]", text.Muted, fingerprint)
			}
			b.WriteString("\n")
		}
	}

	content := b.String()
	// Capture previous scroll position to avoid fighting user scrolling.
	prevRow, prevCol := vm.detail.GetScrollOffset()
	itemChanged := vm.lastDetailID != item.ID
	vm.detail.SetText(content)
	vm.scrollDetailToActive(content, itemChanged, prevRow, prevCol)
	vm.lastDetailID = item.ID
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

func (vm *viewModel) formatEpisodeLine(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int, active bool, stageName string) string {
	label := formatEpisodeLabel(ep)
	stage := vm.episodeStageChip(stageName)
	title, extra, _ := vm.describeEpisode(ep, titles, keyLookup)

	marker := fmt.Sprintf("[%s]·[-]", vm.theme.Text.Faint)
	if active {
		marker = fmt.Sprintf("[%s::b]>[-]", vm.theme.Text.Accent)
	}

	// Trim extras to a short set
	if len(extra) > 2 {
		extra = extra[:2]
	}
	extraText := ""
	if len(extra) > 0 {
		extraText = fmt.Sprintf(" [%s](%s)[-]", vm.theme.Text.Faint, strings.Join(extra, " · "))
	}

	titleText := fmt.Sprintf("[%s]%s[-]", vm.theme.Text.Primary, tview.Escape(title))
	if active {
		titleText = fmt.Sprintf("[%s::b]%s[-:-:-]", vm.theme.Text.Primary, tview.Escape(title))
	}

	files := vm.describeEpisodeFileSummary(&ep, stageName)
	if files != "" {
		extraText = strings.TrimSpace(extraText + "  " + files)
	}
	return fmt.Sprintf("%s [%s]%s[-] %s %s%s\n", marker, vm.theme.Text.Muted, label, stage, titleText, extraText)
}

func (vm *viewModel) describeEpisode(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) (string, []string, *spindle.RipSpecTitleInfo) {
	title := strings.TrimSpace(ep.Title)
	if title == "" {
		title = strings.TrimSpace(ep.OutputBasename)
	}
	if title == "" {
		title = strings.TrimSpace(ep.SourceTitle)
	}
	if title == "" {
		title = "Unlabeled"
	}
	extra := []string{}
	if runtime := formatRuntime(ep.RuntimeSeconds); runtime != "" {
		extra = append(extra, runtime)
	}
	if lang := strings.TrimSpace(ep.SubtitleLanguage); lang != "" {
		extra = append(extra, strings.ToUpper(lang))
	}
	if ep.MatchScore > 0 {
		extra = append(extra, fmt.Sprintf("score %.2f", ep.MatchScore))
	}
	if base := strings.TrimSpace(ep.OutputBasename); base != "" && !strings.EqualFold(base, title) {
		extra = append(extra, base)
	}
	info := vm.lookupRipTitleInfo(ep, titles, keyLookup)
	if info != nil {
		titleID := ep.SourceTitleID
		if titleID <= 0 {
			titleID = info.ID
		}
		if titleID > 0 {
			extra = append(extra, fmt.Sprintf("T%02d", titleID))
		}
		playlist := strings.TrimSpace(info.Playlist)
		if playlist != "" {
			extra = append(extra, fmt.Sprintf("mpls %s", playlist))
		}
	}
	return title, extra, info
}

func (vm *viewModel) formatEpisodeFocusLine(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int, stageName string) string {
	stage := vm.episodeStageChip(stageName)
	title, extras, _ := vm.describeEpisode(ep, titles, keyLookup)
	if len(extras) > 2 {
		extras = extras[:2]
	}
	extraText := ""
	if len(extras) > 0 {
		extraText = fmt.Sprintf(" [%s](%s)[-]", vm.theme.Text.Faint, strings.Join(extras, " · "))
	}
	return fmt.Sprintf("%s [%s::b]%s[-:-:-]%s", stage, vm.theme.Text.Primary, tview.Escape(title), extraText)
}

func (vm *viewModel) describeEpisodeTrackInfo(ep *spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) string {
	info := vm.lookupRipTitleInfo(*ep, titles, keyLookup)
	parts := []string{}
	if info != nil {
		if info.ID > 0 {
			parts = append(parts, fmt.Sprintf("Title %02d", info.ID))
		}
		if name := strings.TrimSpace(info.Name); name != "" {
			parts = append(parts, name)
		}
		if playlist := strings.TrimSpace(info.Playlist); playlist != "" {
			parts = append(parts, fmt.Sprintf("mpls %s", playlist))
		}
		if info.Duration > 0 {
			parts = append(parts, formatRuntime(info.Duration))
		}
	}
	if info == nil && ep.SourceTitleID > 0 {
		parts = append(parts, fmt.Sprintf("Title %02d", ep.SourceTitleID))
	}
	if source := strings.TrimSpace(ep.SourceTitle); source != "" {
		parts = append(parts, source)
	}
	if key := strings.TrimSpace(ep.Key); key != "" {
		parts = append(parts, strings.ToUpper(key))
	}
	if len(parts) == 0 {
		return ""
	}
	for i := range parts {
		parts[i] = fmt.Sprintf("[%s]%s[-]", vm.theme.Text.Accent, tview.Escape(parts[i]))
	}
	return strings.Join(parts, "  ")
}

func (vm *viewModel) movieFocusLine(summary spindle.RipSpecSummary, stage string) string {
	if len(summary.Titles) == 0 {
		return ""
	}
	var main *spindle.RipSpecTitleInfo
	for i := range summary.Titles {
		t := summary.Titles[i]
		if main == nil || t.Duration > main.Duration {
			main = &t
		}
	}
	if main == nil {
		return ""
	}
	name := strings.TrimSpace(main.Name)
	if name == "" {
		name = fmt.Sprintf("Title %02d", main.ID)
	}
	bits := []string{}
	if main.Duration > 0 {
		bits = append(bits, formatRuntime(main.Duration))
	}
	if playlist := strings.TrimSpace(main.Playlist); playlist != "" {
		bits = append(bits, fmt.Sprintf("mpls %s", playlist))
	}
	stageChip := vm.episodeStageChip(stage)
	extra := ""
	if len(bits) > 0 {
		extra = fmt.Sprintf(" [%s](%s)[-]", vm.theme.Text.Faint, strings.Join(bits, " · "))
	}
	return fmt.Sprintf("%s [%s::b]%s[-:-:-]%s", stageChip, vm.theme.Text.Primary, tview.Escape(name), extra)
}

func (vm *viewModel) describeEpisodeFileStates(ep *spindle.EpisodeStatus) string {
	type fileState struct {
		label string
		path  string
		stage string
	}
	states := []fileState{
		{"Rip", ep.RippedPath, "ripping"},
		{"Encode", ep.EncodedPath, "encoding"},
		{"Final", ep.FinalPath, "final"},
	}
	parts := make([]string, 0, len(states))
	for _, st := range states {
		value := "–"
		switch strings.TrimSpace(st.path) {
		case "":
			if normalizeEpisodeStage(ep.Stage) == st.stage {
				value = "…"
			}
		default:
			value = "✓"
		}
		parts = append(parts, fmt.Sprintf("%s %s", st.label, value))
	}
	return fmt.Sprintf("[%s]%s[-]", vm.theme.Text.Faint, strings.Join(parts, "  "))
}

// describeEpisodeFileSummary is a lighter version for list rows.
func (vm *viewModel) describeEpisodeFileSummary(ep *spindle.EpisodeStatus, stageName string) string {
	stage := normalizeEpisodeStage(stageName)
	if stage == "" {
		stage = normalizeEpisodeStage(ep.Stage)
	}
	flags := []string{}
	add := func(label, path, matchStage string) {
		value := "–"
		if strings.TrimSpace(path) != "" {
			value = "✓"
		} else if matchStage == stage {
			value = "…"
		}
		flags = append(flags, fmt.Sprintf("%s %s", label, value))
	}
	add("Rip", ep.RippedPath, "ripping")
	add("Enc", ep.EncodedPath, "encoding")
	add("Fin", ep.FinalPath, "final")
	return fmt.Sprintf("[%s]%s[-]", vm.theme.Text.Faint, strings.Join(flags, "  "))
}

func (vm *viewModel) describeItemFileStates(item spindle.QueueItem, stage string) string {
	current := normalizeEpisodeStage(stage)
	entries := []struct {
		label  string
		path   string
		expect string
	}{
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
		parts = append(parts, fmt.Sprintf("%s %s", e.label, symbol))
	}
	return fmt.Sprintf("[%s]%s[-]", vm.theme.Text.Faint, strings.Join(parts, "  "))
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

func (vm *viewModel) describeEpisodeSubtitleInfo(ep *spindle.EpisodeStatus) string {
	source := strings.TrimSpace(ep.SubtitleSource)
	lang := strings.TrimSpace(ep.SubtitleLanguage)
	if source == "" && lang == "" {
		return ""
	}
	parts := []string{}
	if lang != "" {
		parts = append(parts, strings.ToUpper(lang))
	}
	if source != "" {
		parts = append(parts, titleCase(source))
	}
	return fmt.Sprintf("[%s]%s[-]", vm.theme.Text.AccentSoft, strings.Join(parts, " · "))
}

func (vm *viewModel) episodeStage(ep spindle.EpisodeStatus, fallback string) string {
	stage := normalizeEpisodeStage(ep.Stage)
	if stage == "" {
		stage = normalizeEpisodeStage(fallback)
	}
	if stage == "" {
		stage = "planned"
	}
	return stage
}

func (vm *viewModel) activeEpisodeIndex(item spindle.QueueItem, episodes []spindle.EpisodeStatus) int {
	if len(episodes) == 0 {
		return -1
	}
	target := normalizeEpisodeStage(item.Progress.Stage)
	if target == "" {
		target = normalizeEpisodeStage(item.Status)
	}
	if idx := firstEpisodeWithStage(episodes, target); idx >= 0 {
		return idx
	}
	bestIdx := -1
	bestRank := 1 << 30
	for i, ep := range episodes {
		stage := normalizeEpisodeStage(ep.Stage)
		if stage == "" {
			stage = "planned"
		}
		if stage == "completed" || (stage == "final" && strings.TrimSpace(ep.FinalPath) != "") {
			continue
		}
		rank := episodeStageRank(stage)
		if rank < bestRank {
			bestRank = rank
			bestIdx = i
		}
	}
	if bestIdx >= 0 {
		return bestIdx
	}
	return 0
}

func firstEpisodeWithStage(episodes []spindle.EpisodeStatus, stage string) int {
	normalized := normalizeEpisodeStage(stage)
	if normalized == "" {
		return -1
	}
	for i, ep := range episodes {
		if normalizeEpisodeStage(ep.Stage) == normalized {
			return i
		}
	}
	return -1
}

func normalizeEpisodeStage(stage string) string {
	s := strings.ToLower(strings.TrimSpace(stage))
	if s == "" {
		return ""
	}
	switch s {
	case "pending", "planned", "queued":
		return "planned"
	case "identify", "identifying":
		return "identifying"
	case "rip", "ripping":
		return "ripping"
	case "ripped", "rip_complete":
		return "ripped"
	case "encode", "encoding":
		return "encoding"
	case "encoded", "encode_complete":
		return "encoded"
	case "subtitle", "subtitles", "subtitling":
		return "subtitling"
	case "subtitled":
		return "subtitled"
	case "organizing", "organize":
		return "organizing"
	case "final", "finalizing":
		return "final"
	case "completed", "complete":
		return "completed"
	case "review":
		return "review"
	case "failed", "fail", "error":
		return "failed"
	}
	switch {
	case strings.Contains(s, "identif"):
		return "identifying"
	case strings.Contains(s, "rip"):
		if strings.Contains(s, "done") || strings.Contains(s, "complete") {
			return "ripped"
		}
		return "ripping"
	case strings.Contains(s, "encode"):
		if strings.Contains(s, "done") || strings.Contains(s, "complete") {
			return "encoded"
		}
		return "encoding"
	case strings.Contains(s, "subtit"):
		if strings.Contains(s, "done") || strings.Contains(s, "ready") {
			return "subtitled"
		}
		return "subtitling"
	case strings.Contains(s, "organ"):
		return "organizing"
	case strings.Contains(s, "final"):
		return "final"
	case strings.Contains(s, "review"):
		return "review"
	case strings.Contains(s, "fail") || strings.Contains(s, "error"):
		return "failed"
	}
	return s
}

func episodeStageRank(stage string) int {
	switch normalizeEpisodeStage(stage) {
	case "planned":
		return 1
	case "identifying":
		return 2
	case "ripping":
		return 3
	case "ripped":
		return 4
	case "encoding":
		return 5
	case "encoded":
		return 6
	case "subtitling":
		return 7
	case "subtitled":
		return 8
	case "organizing":
		return 9
	case "final":
		return 10
	case "completed":
		return 11
	case "review":
		return 12
	case "failed":
		return 13
	default:
		return 99
	}
}

func (vm *viewModel) lookupRipTitleInfo(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) *spindle.RipSpecTitleInfo {
	if len(titles) == 0 {
		return nil
	}
	if info, ok := titles[ep.SourceTitleID]; ok && info != nil {
		return info
	}
	key := strings.ToLower(strings.TrimSpace(ep.Key))
	if key == "" {
		return nil
	}
	if id, ok := keyLookup[key]; ok {
		if info, ok := titles[id]; ok {
			return info
		}
	}
	return nil
}

func (vm *viewModel) describeEpisodeTotals(list []spindle.EpisodeStatus, totals spindle.EpisodeTotals) string {
	if totals.Planned == 0 {
		return ""
	}
	stageCounts := map[string]int{}
	for _, ep := range list {
		stage := normalizeEpisodeStage(ep.Stage)
		stageCounts[stage]++
	}
	inFlight := 0
	for stage, count := range stageCounts {
		switch stage {
		case "final", "completed":
			// not in flight
		default:
			if stage != "" {
				inFlight += count
			}
		}
	}
	parts := []string{fmt.Sprintf("%d planned", totals.Planned)}
	if r := chooseMax(totals.Ripped, stageCounts["ripping"]+stageCounts["ripped"]); r > 0 {
		parts = append(parts, fmt.Sprintf("%d ripping", r))
	}
	if e := chooseMax(totals.Encoded, stageCounts["encoding"]+stageCounts["encoded"]); e > 0 {
		parts = append(parts, fmt.Sprintf("%d encoding", e))
	}
	if s := stageCounts["subtitling"] + stageCounts["subtitled"]; s > 0 {
		parts = append(parts, fmt.Sprintf("%d subtitling", s))
	}
	if o := stageCounts["organizing"]; o > 0 {
		parts = append(parts, fmt.Sprintf("%d organizing", o))
	}
	if f := chooseMax(totals.Final, stageCounts["final"]+stageCounts["completed"]); f > 0 {
		parts = append(parts, fmt.Sprintf("%d final", f))
	}
	if inFlight > 0 {
		parts = append(parts, fmt.Sprintf("%d in flight", inFlight))
	}
	return strings.Join(parts, " · ")
}

func chooseMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (vm *viewModel) episodeStageChip(stage string) string {
	status := stageToStatus(stage)
	color := vm.colorForStatus(status)
	label := strings.ToUpper(strings.TrimSpace(stage))
	if label == "" {
		label = "PLANNED"
	}
	return fmt.Sprintf("[%s:%s] %s [-:-]", vm.theme.Base.Background, color, label)
}

func stageToStatus(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "ripping":
		return "ripping"
	case "encoding":
		return "encoding"
	case "subtitling":
		return "encoding"
	case "subtitled":
		return "encoded"
	case "final":
		return "completed"
	case "encoded":
		return "encoded"
	case "ripped":
		return "ripped"
	case "organizing":
		return "organizing"
	case "review":
		return "review"
	case "failed":
		return "failed"
	default:
		return "pending"
	}
}

func formatEpisodeLabel(ep spindle.EpisodeStatus) string {
	if ep.Season > 0 && ep.Episode > 0 {
		return fmt.Sprintf("S%02dE%02d", ep.Season, ep.Episode)
	}
	if strings.TrimSpace(ep.Key) != "" {
		return strings.ToUpper(ep.Key)
	}
	return "EP"
}

func formatRuntime(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	minutes := seconds / 60
	if minutes == 0 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (vm *viewModel) detailProgressBar(item spindle.QueueItem) string {
	stage := normalizeEpisodeStage(item.Progress.Stage)
	if stage == "" {
		stage = normalizeEpisodeStage(item.Status)
	}
	percent := clampPercent(item.Progress.Percent)
	line := fmt.Sprintf("%s  [%s]%3.0f%%[-]", vm.detailStageLadder(stage), vm.theme.Text.Muted, percent)
	if eta := vm.estimateETA(item); eta != "" {
		line += fmt.Sprintf("  [%s]%s[-]", vm.theme.Text.Secondary, eta)
	}
	return line
}

func (vm *viewModel) detailStageLadder(stage string) string {
	current := normalizeEpisodeStage(stage)
	currentRank := episodeStageRank(current)
	if currentRank == 0 || currentRank >= 99 {
		currentRank = episodeStageRank("planned")
	}
	stages := []struct {
		name  string
		label string
	}{
		{"planned", "PLAN"},
		{"ripping", "RIP"},
		{"encoding", "ENC"},
		{"final", "FINAL"},
		{"completed", "DONE"},
	}
	parts := make([]string, 0, len(stages))
	for _, st := range stages {
		rank := episodeStageRank(st.name)
		symbol := "·"
		color := vm.theme.Text.Faint
		switch {
		case currentRank > rank:
			symbol = "✓"
			color = vm.theme.Text.Success
		case currentRank == rank:
			symbol = "…"
			color = vm.colorForStatus(stageToStatus(st.name))
		}
		parts = append(parts, fmt.Sprintf("[%s]%s %s[-]", color, symbol, st.label))
	}
	return strings.Join(parts, "  ")
}

func (vm *viewModel) estimateETA(item spindle.QueueItem) string {
	stage := normalizeEpisodeStage(item.Progress.Stage)
	if stage == "" {
		stage = normalizeEpisodeStage(item.Status)
	}
	if enc := item.Encoding; enc != nil && (stage == "encoding" || stage == "encoded" || stage == "final") {
		if eta := enc.ETADuration(); eta > 0 {
			return "ETA " + humanizeDuration(eta)
		}
	}
	percent := clampPercent(item.Progress.Percent)
	if percent <= 1 || percent >= 100 {
		return ""
	}
	start := item.ParsedCreatedAt()
	if start.IsZero() {
		return ""
	}
	elapsed := time.Since(start)
	if elapsed <= 0 {
		return ""
	}
	remaining := time.Duration(float64(elapsed) * (100 - percent) / percent)
	if remaining <= 0 {
		return ""
	}
	return "ETA " + humanizeDuration(remaining)
}

func (vm *viewModel) scrollDetailToActive(content string, itemChanged bool, prevRow, prevCol int) {
	if vm.detail == nil {
		return
	}
	// Only auto-scroll when selection changed or user was at the top.
	if !itemChanged && (prevRow > 0 || prevCol > 0) {
		return
	}
	lines := strings.Split(content, "\n")
	target := -1
	for i, line := range lines {
		if strings.Contains(line, "NOW") {
			target = i
			break
		}
	}
	if target >= 0 {
		vm.detail.ScrollTo(target, 0)
	} else {
		vm.detail.ScrollToBeginning()
	}
}

func formatEncodingMetrics(enc *spindle.EncodingStatus) string {
	if enc == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if eta := enc.ETADuration(); eta > 0 {
		parts = append(parts, fmt.Sprintf("ETA %s", humanizeDuration(eta)))
	}
	if enc.Speed > 0 {
		parts = append(parts, fmt.Sprintf("%.1fx", enc.Speed))
	}
	if enc.FPS > 0 {
		parts = append(parts, fmt.Sprintf("%.0f fps", enc.FPS))
	}
	if frames := formatEncodingFrameSummary(enc); frames != "" {
		parts = append(parts, frames)
	}
	if bitrate := strings.TrimSpace(enc.Bitrate); bitrate != "" {
		parts = append(parts, bitrate)
	}
	return strings.Join(parts, " • ")
}

func hasEncodingDetails(enc *spindle.EncodingStatus) bool {
	if enc == nil {
		return false
	}
	if enc.Hardware != nil || enc.Config != nil || enc.Video != nil || enc.Crop != nil || enc.Validation != nil || enc.Result != nil || strings.TrimSpace(enc.Warning) != "" || enc.Error != nil {
		return true
	}
	return false
}

func formatEncodingHardware(enc *spindle.EncodingStatus) string {
	if enc == nil || enc.Hardware == nil {
		return ""
	}
	return strings.TrimSpace(enc.Hardware.Hostname)
}

func formatEncodingConfig(enc *spindle.EncodingStatus) string {
	if enc == nil || enc.Config == nil {
		return ""
	}
	cfg := enc.Config
	parts := make([]string, 0, 4)
	if val := strings.TrimSpace(cfg.Encoder); val != "" {
		parts = append(parts, val)
	}
	if val := strings.TrimSpace(cfg.Quality); val != "" {
		parts = append(parts, val)
	}
	if val := strings.TrimSpace(cfg.Preset); val != "" {
		parts = append(parts, fmt.Sprintf("Preset %s", val))
	}
	if val := strings.TrimSpace(cfg.Tune); val != "" {
		parts = append(parts, fmt.Sprintf("Tune %s", val))
	}
	if val := strings.TrimSpace(cfg.AudioCodec); val != "" {
		parts = append(parts, fmt.Sprintf("Audio %s", val))
	}
	return strings.Join(parts, " • ")
}

func formatEncodingSource(enc *spindle.EncodingStatus) string {
	if enc == nil || enc.Video == nil {
		return ""
	}
	video := enc.Video
	parts := make([]string, 0, 3)
	if res := strings.TrimSpace(video.Resolution); res != "" {
		category := strings.TrimSpace(video.Category)
		if category != "" {
			res = fmt.Sprintf("%s (%s)", res, category)
		}
		parts = append(parts, res)
	}
	if dur := strings.TrimSpace(video.Duration); dur != "" {
		parts = append(parts, dur)
	}
	if audio := strings.TrimSpace(video.AudioDescription); audio != "" {
		parts = append(parts, audio)
	}
	return strings.Join(parts, " • ")
}

func formatEncodingCrop(enc *spindle.EncodingStatus) string {
	if enc == nil || enc.Crop == nil {
		return ""
	}
	crop := enc.Crop
	summary := strings.TrimSpace(crop.Message)
	params := strings.TrimSpace(crop.Crop)
	if params != "" {
		if summary != "" {
			summary = fmt.Sprintf("%s (%s)", summary, params)
		} else {
			summary = params
		}
	}
	flags := make([]string, 0, 2)
	if crop.Required {
		flags = append(flags, "required")
	}
	if crop.Disabled {
		flags = append(flags, "disabled")
	}
	if len(flags) > 0 {
		summary = strings.TrimSpace(summary + " [" + strings.Join(flags, ", ") + "]")
	}
	return summary
}

func formatEncodingValidation(enc *spindle.EncodingStatus) string {
	if enc == nil || enc.Validation == nil {
		return ""
	}
	if enc.Validation.Passed {
		return "Passed"
	}
	for _, step := range enc.Validation.Steps {
		if !step.Passed {
			name := strings.TrimSpace(step.Name)
			if name == "" {
				name = "validation"
			}
			return fmt.Sprintf("Failed: %s", name)
		}
	}
	return "Failed"
}

func formatEncodingResult(enc *spindle.EncodingStatus) string {
	if enc == nil || enc.Result == nil {
		return ""
	}
	res := enc.Result
	parts := make([]string, 0, 4)
	if res.OriginalSize > 0 || res.EncodedSize > 0 {
		size := fmt.Sprintf("%s → %s", formatBytes(res.OriginalSize), formatBytes(res.EncodedSize))
		if res.SizeReductionPercent != 0 {
			size = fmt.Sprintf("%s (%.1f%%)", size, res.SizeReductionPercent)
		}
		parts = append(parts, size)
	}
	if res.AverageSpeed > 0 {
		parts = append(parts, fmt.Sprintf("avg %.1fx", res.AverageSpeed))
	}
	streams := strings.TrimSpace(strings.Join(filterNonEmpty(
		strings.TrimSpace(res.VideoStream),
		strings.TrimSpace(res.AudioStream),
	), " / "))
	if streams != "" {
		parts = append(parts, streams)
	}
	return strings.Join(parts, " • ")
}

func formatEncodingFrameSummary(enc *spindle.EncodingStatus) string {
	if enc == nil || enc.TotalFrames <= 0 || enc.CurrentFrame <= 0 {
		return ""
	}
	percent := int(math.Round(enc.FramePercent() * 100))
	return fmt.Sprintf("%d/%d frames (%d%%)", enc.CurrentFrame, enc.TotalFrames, percent)
}

func formatEncodingIssue(issue *spindle.EncodingIssue) string {
	if issue == nil {
		return ""
	}
	title := strings.TrimSpace(issue.Title)
	message := strings.TrimSpace(issue.Message)
	switch {
	case title != "" && message != "":
		return fmt.Sprintf("%s – %s", title, message)
	case message != "":
		return message
	default:
		return title
	}
}

func filterNonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, strings.TrimSpace(v))
		}
	}
	return out
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

func (vm *viewModel) logPreview(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	entry := vm.logPreviewCache[path]
	if entry.text != "" && time.Since(entry.readAt) < 3*time.Second {
		return entry.text
	}
	lines, err := logtail.Read(path, 12)
	if err != nil || len(lines) == 0 {
		entry.text = ""
		entry.readAt = time.Now()
		vm.logPreviewCache[path] = entry
		return ""
	}
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		entry.text = truncate(line, 120)
		break
	}
	entry.readAt = time.Now()
	vm.logPreviewCache[path] = entry
	return entry.text
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

func padLabel(label string) string {
	return fmt.Sprintf("%-*s", detailLabelWidth, label+":")
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
