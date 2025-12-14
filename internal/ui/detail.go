package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

func (vm *viewModel) updateDetail(row int) {
	if row <= 0 || row-1 >= len(vm.displayItems) {
		vm.detail.SetText(fmt.Sprintf("[%s]Select an item to view details[-]", vm.theme.Text.Faint))
		vm.lastDetailID = 0
		return
	}
	item := vm.displayItems[row-1]

	// Pre-calculation
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
	mediaType := detectMediaType(item.Metadata)

	var b strings.Builder
	text := vm.theme.Text
	now := time.Now()

	// -- HEADER --
	title := composeTitle(item)
	fmt.Fprintf(&b, "[%s]%s[-]\n", text.Heading, tview.Escape(title))

	formatStamp := func(ts time.Time) string {
		if ts.IsZero() {
			return ""
		}
		if ts.Year() == now.Year() && ts.YearDay() == now.YearDay() {
			return ts.Format("15:04:05")
		}
		return ts.Format("Jan 02 15:04")
	}

	metaParts := []string{fmt.Sprintf("[%s]#%d[-]", text.Muted, item.ID)}
	if created := item.ParsedCreatedAt(); !created.IsZero() {
		metaParts = append(metaParts, fmt.Sprintf("[%s]created[-] [%s]%s[-]", text.Faint, text.Secondary, formatStamp(created)))
	}
	if updated := item.ParsedUpdatedAt(); !updated.IsZero() {
		ago := now.Sub(updated)
		if ago < 0 {
			ago = 0
		}
		metaParts = append(metaParts, fmt.Sprintf("[%s]updated[-] [%s]%s[-] [%s](%s)[-]", text.Faint, text.Secondary, formatStamp(updated), text.Muted, humanizeDuration(ago)))
	}
	fmt.Fprintf(&b, "%s\n", strings.Join(metaParts, fmt.Sprintf(" [%s]•[-] ", text.Faint)))

	// Status & Chips
	status := vm.statusChip(item.Status)
	lane := vm.laneChip(determineLane(item))
	chips := []string{status}
	if lane != "" {
		chips = append(chips, lane)
	}
	if item.NeedsReview {
		chips = append(chips, vm.badge("REVIEW", vm.theme.Badges.Review))
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		chips = append(chips, vm.badge("ERROR", vm.theme.Badges.Error))
	}
	if isRipCacheHitMessage(item.Progress.Message) {
		chips = append(chips, vm.badge("CACHE", vm.theme.Badges.Info))
	}
	if item.SubtitleGeneration != nil && item.SubtitleGeneration.FallbackUsed {
		label := "AI"
		if item.SubtitleGeneration.WhisperX > 1 {
			label = fmt.Sprintf("AI%d", item.SubtitleGeneration.WhisperX)
		}
		chips = append(chips, vm.badge(label, vm.theme.Badges.Fallback))
	}
	if preset := item.DraptoPresetLabel(); preset != "" {
		chips = append(chips, vm.badge(strings.ToUpper(preset), vm.theme.StatusColor("pending")))
	}
	fmt.Fprintf(&b, "%s\n", strings.Join(chips, " "))

	// -- PIPELINE --
	fmt.Fprint(&b, "\n")
	vm.renderPipelineStatus(&b, item, totals)
	fmt.Fprint(&b, "\n")

	// -- ACTIVE PROGRESS --
	vm.renderActiveProgress(&b, item)

	writeSection := func(title string) {
		fmt.Fprintf(&b, "\n[%s]%s[-]\n", text.Secondary, strings.ToUpper(title))
		fmt.Fprintf(&b, "[%s]%s[-]\n", text.Faint, strings.Repeat("─", 38))
	}

	// -- ISSUES --
	if item.NeedsReview || strings.TrimSpace(item.ErrorMessage) != "" {
		writeSection("Attention")
		if item.NeedsReview {
			reason := strings.TrimSpace(item.ReviewReason)
			if reason == "" {
				reason = "Needs operator review"
			}
			fmt.Fprintf(&b, "[%s]Review:[-]  [%s]%s[-]\n", text.Warning, text.Primary, tview.Escape(reason))
		}
		if msg := strings.TrimSpace(item.ErrorMessage); msg != "" {
			fmt.Fprintf(&b, "[%s]Error:[-]   [%s]%s[-]\n", text.Danger, text.Primary, tview.Escape(msg))
		}
	}

	// -- PROGRESS / NOW --
	currentStage := normalizeEpisodeStage(item.Progress.Stage)
	if currentStage == "" {
		currentStage = normalizeEpisodeStage(item.Status)
	}

	if len(episodes) > 0 && mediaType != "movie" {
		// TV Show focus
		activeIdx := vm.activeEpisodeIndex(item, episodes)
		if activeIdx >= 0 && activeIdx < len(episodes) {
			ep := episodes[activeIdx]
			writeSection("Current Episode")
			stage := vm.episodeStage(ep, currentStage, true)
			fmt.Fprint(&b, vm.formatEpisodeFocusLine(ep, titleLookup, episodeTitleIndex, stage))
			b.WriteString("\n")
			if track := vm.describeEpisodeTrackInfo(&ep, titleLookup, episodeTitleIndex); track != "" {
				fmt.Fprintf(&b, "[%s]Track:[-]   %s\n", text.Muted, track)
			}
			if files := vm.describeEpisodeFileStates(&ep); files != "" {
				fmt.Fprintf(&b, "[%s]Files:[-]   %s\n", text.Muted, files)
			}
		}
	} else {
		// Movie focus
		if cut := vm.movieFocusLine(summary, currentStage); cut != "" {
			writeSection("Current Selection")
			fmt.Fprint(&b, cut)
			b.WriteString("\n")
			if files := vm.describeItemFileStates(item, currentStage); files != "" {
				fmt.Fprintf(&b, "[%s]Files:[-]   %s\n", text.Muted, files)
			}
		}
	}

	// -- PATHS --
	expandedPaths := vm.pathsExpanded(item.ID)
	writePath := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if !expandedPaths {
			value = truncateMiddle(value, 64)
		}
		fmt.Fprintf(&b, "[%s]%s[-]  [%s]%s[-]\n", text.Muted, padRight(label, 8), text.Secondary, tview.Escape(value))
	}

	hasAnyPaths := strings.TrimSpace(item.SourcePath) != "" ||
		strings.TrimSpace(item.BackgroundLogPath) != ""

	if hasAnyPaths {
		writeSection("Paths")
		writePath("Source:", item.SourcePath)
		writePath("Log:", item.BackgroundLogPath)
	}

	// -- EPISODES LIST --
	if len(episodes) > 0 && mediaType != "movie" {
		writeSection("Episodes")
		if !item.EpisodesSynced {
			fmt.Fprintf(&b, "[%s]Episode numbers not confirmed[-]\n", vm.theme.Text.Warning)
		}

		collapsed := vm.episodesCollapsed(item.ID)
		if collapsed {
			summary := vm.describeEpisodeTotals(episodes, totals)
			// Add a mini progress bar showing overall completion
			if totals.Planned > 0 {
				percent := (float64(totals.Final) / float64(totals.Planned)) * 100
				bar := vm.drawProgressBar(percent, 20, vm.theme.StatusColor("completed"))
				fmt.Fprintf(&b, "[%s]%s[-]  %s %.0f%%\n", text.Primary, summary, bar, percent)
				fmt.Fprintf(&b, "[%s](press t to expand)[-]\n", text.Faint)
			} else {
				fmt.Fprintf(&b, "[%s]%s[-]  [%s](press t to expand)[-]\n", text.Primary, summary, text.Faint)
			}
		} else {
			activeIdx := vm.activeEpisodeIndex(item, episodes)
			for idx, ep := range episodes {
				stage := vm.episodeStage(ep, currentStage, idx == activeIdx)
				b.WriteString(vm.formatEpisodeLine(ep, titleLookup, episodeTitleIndex, idx == activeIdx, stage))
			}
			fmt.Fprintf(&b, "[%s]Press t to collapse[-]\n", text.Faint)
		}
	}

	// -- METADATA --
	if metaRows := summarizeMetadata(item.Metadata); len(metaRows) > 0 {
		writeSection("Metadata")
		b.WriteString(vm.formatMetadata(metaRows))
	}

	// -- TECH --
	if enc := item.Encoding; hasEncodingDetails(enc) {
		writeSection("Encoding Stats")
		// Compact stats
		parts := []string{}
		if enc.Speed > 0 {
			parts = append(parts, fmt.Sprintf("%.2fx", enc.Speed))
		}
		if enc.FPS > 0 {
			parts = append(parts, fmt.Sprintf("%.0f fps", enc.FPS))
		}
		if enc.Bitrate != "" {
			parts = append(parts, enc.Bitrate)
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "[%s]Perf:[-]    [%s]%s[-]\n", text.Muted, text.Accent, strings.Join(parts, "  "))
		}

		if result := formatEncodingResult(enc); result != "" {
			fmt.Fprintf(&b, "[%s]Result:[-]  [%s]%s[-]\n", text.Muted, text.Accent, result)
		}
	}

	content := b.String()
	prevRow, prevCol := vm.detail.GetScrollOffset()
	itemChanged := vm.lastDetailID != item.ID
	vm.detail.SetText(content)
	vm.scrollDetailToActive(content, itemChanged, prevRow, prevCol)
	vm.lastDetailID = item.ID
}

func (vm *viewModel) renderPipelineStatus(b *strings.Builder, item spindle.QueueItem, totals spindle.EpisodeTotals) {
	text := vm.theme.Text

	// Stages: Planned -> Ripped -> Encoded -> Subtitled -> Final
	stages := []struct {
		id    string
		label string
	}{
		{"planned", "Planned"},
		{"ripped", "Ripped"},
		{"encoded", "Encoded"},
		{"subtitled", "Subtitled"},
		{"final", "Final"},
	}

	activeStage := normalizeEpisodeStage(item.Progress.Stage)
	if activeStage == "" {
		activeStage = normalizeEpisodeStage(item.Status)
	}
	if activeStage == "" {
		activeStage = "planned"
	}
	activePipelineStage := pipelineStageForStatus(activeStage)

	plannedCount := totals.Planned
	if plannedCount <= 0 {
		plannedCount = 1
	}

	// For TV episode items, derive subtitle completion by inspecting episode stages.
	subtitledCount := 0
	if totals.Planned > 0 {
		episodes, _ := item.EpisodeSnapshot()
		for _, ep := range episodes {
			if normalizeEpisodeStage(ep.Stage) == "subtitled" || normalizeEpisodeStage(ep.Stage) == "final" {
				subtitledCount++
			}
		}
	}

	for i, stage := range stages {
		if i > 0 {
			fmt.Fprintf(b, " [%s]→[-] ", text.Faint)
		}

		count := plannedCount
		if totals.Planned > 0 {
			switch stage.id {
			case "planned":
				count = totals.Planned
			case "ripped":
				count = totals.Ripped
			case "encoded":
				count = totals.Encoded
			case "subtitled":
				count = subtitledCount
			case "final":
				count = totals.Final
			}
		} else {
			count = singleItemPipelineCount(stage.id, item, activeStage, plannedCount)
		}

		isComplete := count >= plannedCount
		isCurrent := !isComplete && stage.id == activePipelineStage

		// Unified rendering: checkmarks + current indicator for both TV and movies.
		icon := "○"
		color := text.Muted
		labelColor := text.Secondary
		labelStyle := ""
		switch {
		case isComplete:
			icon = "✓"
			color = vm.theme.StatusColor("completed")
			labelStyle = "::b"
		case isCurrent:
			icon = "◉"
			color = vm.theme.Text.AccentSoft
			labelColor = vm.theme.Text.Accent
			labelStyle = "::b"
		case count > 0:
			icon = "◐"
			color = vm.theme.Text.Warning
		}

		if plannedCount > 1 {
			fmt.Fprintf(b, "[%s]%s[-] [%s%s]%s[-] [%s]%d/%d[-]",
				color, icon,
				labelColor, labelStyle, stage.label,
				text.Muted, count, plannedCount)
		} else {
			fmt.Fprintf(b, "[%s]%s[-] [%s%s]%s[-]",
				color, icon,
				labelColor, labelStyle, stage.label)
		}
	}
}

// -- Helpers moved and adapted from navigation.go --

func normalizeEpisodeStage(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return ""
	}
	if s == "episode_identifying" || strings.Contains(s, "episode identification") {
		// This stage happens after ripping and before encoding; treat it as "encoding"
		// for the simplified pipeline display.
		return "encoding"
	}
	if s == "episode_identified" || strings.Contains(s, "episode identified") {
		// Episode identification has completed; encoding is the next step.
		return "encoding"
	}
	if s == "subtitled" {
		return "subtitled"
	}
	if strings.HasPrefix(s, "subtitl") {
		return "subtitling"
	}
	if s == "encoded" {
		return "encoded"
	}
	if strings.HasPrefix(s, "encod") {
		return "encoding"
	}
	if s == "ripped" {
		return "ripped"
	}
	if strings.HasPrefix(s, "rip") {
		return "ripping"
	}
	if s == "final" || s == "completed" || s == "complete" || s == "success" || s == "done" || s == "organizing" {
		return "final"
	}
	return "planned"
}

func pipelineStageForStatus(stage string) string {
	switch stage {
	case "ripping", "ripped":
		return "ripped"
	case "encoding", "encoded":
		return "encoded"
	case "subtitling", "subtitled":
		return "subtitled"
	case "final", "completed":
		return "final"
	case "planned", "identifying", "identified", "pending":
		return "planned"
	}
	return "planned"
}

func singleItemPipelineCount(stageID string, item spindle.QueueItem, activeStage string, plannedCount int) int {
	// For a single item, "planned" is always known once it's in the queue.
	if stageID == "planned" {
		return plannedCount
	}

	// Prefer concrete file evidence, then fall back to inferred stage.
	switch stageID {
	case "ripped":
		if strings.TrimSpace(item.RippedFile) != "" {
			return plannedCount
		}
		// Once we've moved past ripping, treat ripped as done.
		switch activeStage {
		case "ripped", "encoding", "encoded", "subtitling", "subtitled", "final":
			return plannedCount
		}
	case "encoded":
		if strings.TrimSpace(item.EncodedFile) != "" {
			return plannedCount
		}
		switch activeStage {
		case "encoded", "subtitling", "subtitled", "final":
			return plannedCount
		}
	case "subtitled":
		switch activeStage {
		case "subtitled", "final":
			return plannedCount
		}
	case "final":
		if strings.TrimSpace(item.FinalFile) != "" {
			return plannedCount
		}
		if activeStage == "final" {
			return plannedCount
		}
	}

	return 0
}

func (vm *viewModel) activeEpisodeIndex(item spindle.QueueItem, episodes []spindle.EpisodeStatus) int {
	if len(episodes) == 0 {
		return -1
	}

	for i, ep := range episodes {
		if ep.Active {
			return i
		}
	}

	// 1. Precise Match: File path
	// If we are ripping, item.RippedFile should match episode.RippedPath (or basename)
	// If we are encoding/subtitling, item.EncodedFile should match episode.EncodedPath
	stage := normalizeEpisodeStage(item.Progress.Stage)
	if stage == "" {
		stage = normalizeEpisodeStage(item.Status)
	}

	checkMatch := func(target, candidate string) bool {
		target = strings.TrimSpace(target)
		candidate = strings.TrimSpace(candidate)
		if target == "" || candidate == "" {
			return false
		}
		// Exact match
		if target == candidate {
			return true
		}
		// Case-insensitive match on full string
		if strings.EqualFold(target, candidate) {
			return true
		}
		// Suffix match (case-insensitive)
		if strings.HasSuffix(strings.ToLower(candidate), strings.ToLower(target)) ||
			strings.HasSuffix(strings.ToLower(target), strings.ToLower(candidate)) {
			return true
		}
		// Base match
		targetBase := filepath.Base(target)
		candidateBase := filepath.Base(candidate)
		if targetBase != "." && candidateBase != "." && strings.EqualFold(targetBase, candidateBase) {
			return true
		}
		return false
	}

	if stage == "ripping" && item.RippedFile != "" {
		for i, ep := range episodes {
			if checkMatch(item.RippedFile, ep.RippedPath) || checkMatch(item.RippedFile, ep.OutputBasename) {
				return i
			}
		}
	}
	if stage == "encoding" || stage == "subtitling" {
		target := item.EncodedFile
		// Also try input match from encoding details
		if target == "" && item.Encoding != nil && item.Encoding.Video != nil {
			target = item.Encoding.Video.OutputFile
		}
		if target != "" {
			for i, ep := range episodes {
				if checkMatch(target, ep.EncodedPath) || checkMatch(target, ep.OutputBasename) {
					return i
				}
			}
		}
		// If we know the input (ripped file), we can match that too
		if item.Encoding != nil && item.Encoding.Video != nil && item.Encoding.Video.InputFile != "" {
			input := item.Encoding.Video.InputFile
			for i, ep := range episodes {
				if checkMatch(input, ep.RippedPath) {
					return i
				}
			}
		}
	}

	// 2. Stage Match: Find first episode explicitly in the current stage
	for i, ep := range episodes {
		if normalizeEpisodeStage(ep.Stage) == stage {
			return i
		}
	}

	// 3. Pipeline Match: Find first episode ready for the current stage
	// If ripping, finding first 'planned'
	// If encoding, find first 'ripped' (waiting for encoding)
	var searchStage string
	switch stage {
	case "ripping":
		searchStage = "planned"
	case "encoding":
		searchStage = "ripped"
	case "subtitling":
		searchStage = "encoded"
	}
	if searchStage != "" {
		for i, ep := range episodes {
			if normalizeEpisodeStage(ep.Stage) == searchStage {
				return i
			}
		}
	}

	// 4. Fallback: First non-final
	for i, ep := range episodes {
		if normalizeEpisodeStage(ep.Stage) != "final" && normalizeEpisodeStage(ep.Stage) != "completed" {
			return i
		}
	}

	return len(episodes) - 1
}

func (vm *viewModel) episodeStage(ep spindle.EpisodeStatus, currentGlobalStage string, isActive bool) string {
	// If the episode is active, and the global stage implies work, override it.
	if isActive {
		// Only override if global stage is a "working" stage.
		// e.g. ripping, encoding, identifying.
		switch currentGlobalStage {
		case "ripping", "encoding", "identifying", "subtitling":
			return currentGlobalStage
		}
	}
	// Otherwise use the episode's intrinsic stage.
	if ep.Stage == "final" {
		return "final"
	}
	return ep.Stage
}

func (vm *viewModel) episodeStageChip(stage string) string {
	stage = strings.ToLower(stage)
	color := vm.theme.Text.Muted
	label := "WAIT"

	switch stage {
	case "final", "completed":
		color = vm.theme.StatusColor("completed")
		label = "DONE"
	case "subtitled":
		color = vm.theme.StatusColor("subtitled")
		label = "SUB"
	case "encoded":
		color = vm.theme.StatusColor("encoded")
		label = "ENCD"
	case "ripped":
		color = vm.theme.StatusColor("ripped")
		label = "RIPD"
	case "planned":
		color = vm.theme.Text.Muted
		label = "PLAN"
	case "encoding":
		color = vm.theme.StatusColor("encoding")
		label = "WORK"
	case "ripping":
		color = vm.theme.StatusColor("ripping")
		label = "WORK"
	case "subtitling":
		color = vm.theme.StatusColor("subtitling")
		label = "WORK"
	}

	return fmt.Sprintf("[%s:%s] %s [-:-]", vm.theme.Base.Background, color, label)
}

func (vm *viewModel) describeEpisodeTotals(episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals) string {
	if len(episodes) == 0 {
		return "No episodes"
	}
	parts := []string{}
	if totals.Final > 0 {
		parts = append(parts, fmt.Sprintf("%d done", totals.Final))
	}
	if totals.Encoded > totals.Final {
		parts = append(parts, fmt.Sprintf("%d encoded", totals.Encoded-totals.Final))
	}
	if totals.Ripped > totals.Encoded {
		parts = append(parts, fmt.Sprintf("%d ripped", totals.Ripped-totals.Encoded))
	}
	rem := totals.Planned - totals.Ripped
	if rem > 0 {
		parts = append(parts, fmt.Sprintf("%d planned", rem))
	}
	return strings.Join(parts, ", ")
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

	return fmt.Sprintf("%s [%s]%s[-] %s %s%s\n", marker, vm.theme.Text.Muted, label, stage, titleText, extraText)
}

func formatEpisodeLabel(ep spindle.EpisodeStatus) string {
	if ep.Season == 0 && ep.Episode == 0 {
		return "S??E??"
	}
	return fmt.Sprintf("S%02dE%02d", ep.Season, ep.Episode)
}

func (vm *viewModel) scrollDetailToActive(content string, itemChanged bool, prevRow, prevCol int) {
	// Basic scroll preservation or reset
	if itemChanged {
		vm.detail.ScrollTo(0, 0)
	} else {
		vm.detail.ScrollTo(prevRow, prevCol)
	}
}

// ... Additional helpers from navigation.go that we need to keep ...
// describeEpisode, lookupRipTitleInfo, etc.
// For brevity in this turn, I will assume I need to copy them here or make them public.
// Since I can't easily make them public in the other file without editing it first, I'll copy the logic.

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
	lang := strings.TrimSpace(ep.GeneratedSubtitleLanguage)
	if lang == "" {
		lang = strings.TrimSpace(ep.SubtitleLanguage)
	}
	if lang != "" {
		extra = append(extra, strings.ToUpper(lang))
	}
	switch strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleSource)) {
	case "whisperx":
		extra = append(extra, "AI")
		switch strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleDecision)) {
		case "no_match":
			extra = append(extra, "NO-MATCH")
		case "error":
			extra = append(extra, "OS-ERR")
		}
	case "opensubtitles":
		extra = append(extra, "OS")
	}

	info := vm.lookupRipTitleInfo(ep, titles, keyLookup)
	return title, extra, info
}

func (vm *viewModel) lookupRipTitleInfo(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) *spindle.RipSpecTitleInfo {
	if ep.SourceTitleID > 0 {
		return titles[ep.SourceTitleID]
	}
	// Try key lookup
	if key, ok := keyLookup[strings.ToLower(ep.Key)]; ok {
		return titles[key]
	}
	return nil
}

func formatRuntime(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	return fmt.Sprintf("%dm", seconds/60)
}

func (vm *viewModel) describeEpisodeTrackInfo(ep *spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) string {
	info := vm.lookupRipTitleInfo(*ep, titles, keyLookup)
	parts := []string{}
	if info != nil {
		if info.ID > 0 {
			parts = append(parts, fmt.Sprintf("Title %02d", info.ID))
		}
		if info.Duration > 0 {
			parts = append(parts, formatRuntime(info.Duration))
		}
	} else if ep.SourceTitleID > 0 {
		parts = append(parts, fmt.Sprintf("Title %02d", ep.SourceTitleID))
	}
	return strings.Join(parts, "  ")
}

func (vm *viewModel) describeEpisodeFileStates(ep *spindle.EpisodeStatus) string {
	// Simple summary
	parts := []string{}
	if ep.RippedPath != "" {
		parts = append(parts, "[+]Ripped")
	}
	if ep.EncodedPath != "" {
		parts = append(parts, "[+]Encoded")
	}
	if ep.FinalPath != "" {
		parts = append(parts, "[+]Final")
	}
	return strings.Join(parts, " ")
}

func (vm *viewModel) movieFocusLine(summary spindle.RipSpecSummary, stage string) string {
	if len(summary.Titles) == 0 {
		return ""
	}
	main := summary.Titles[0] // Simplify for now
	name := main.Name
	if name == "" {
		name = fmt.Sprintf("Title %02d", main.ID)
	}
	stageChip := vm.episodeStageChip(stage)
	return fmt.Sprintf("%s %s", stageChip, name)
}

func (vm *viewModel) describeItemFileStates(item spindle.QueueItem, stage string) string {
	parts := []string{}
	if item.RippedFile != "" {
		parts = append(parts, "Ripped")
	}
	if item.EncodedFile != "" {
		parts = append(parts, "Encoded")
	}
	if item.FinalFile != "" {
		parts = append(parts, "Final")
	}
	// Use ticks?
	return strings.Join(parts, " · ")
}

func (vm *viewModel) formatEpisodeFocusLine(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int, stageName string) string {
	stage := vm.episodeStageChip(stageName)
	title, _, _ := vm.describeEpisode(ep, titles, keyLookup)
	return fmt.Sprintf("%s [%s::b]%s[-:-:-]", stage, vm.theme.Text.Primary, tview.Escape(title))
}

func hasEncodingDetails(enc *spindle.EncodingStatus) bool {
	return enc != nil && (enc.Speed > 0 || enc.FPS > 0 || enc.Result != nil)
}

func formatEncodingResult(enc *spindle.EncodingStatus) string {
	if enc == nil || enc.Result == nil {
		return ""
	}
	r := enc.Result
	return fmt.Sprintf("size: %s (%.0f%% reduction)", formatBytes(r.EncodedSize), r.SizeReductionPercent)
}

func (vm *viewModel) renderActiveProgress(b *strings.Builder, item spindle.QueueItem) {
	stage := normalizeEpisodeStage(item.Progress.Stage)
	if stage == "" {
		stage = normalizeEpisodeStage(item.Status)
	}

	percent := clampPercent(item.Progress.Percent)
	label := ""
	color := ""
	icon := ""

	switch stage {
	case "ripping":
		label = "RIPPING"
		icon = "⏵"
		color = vm.theme.StatusColor("ripping")
	case "encoding":
		label = "ENCODING"
		icon = "⚙"
		color = vm.theme.StatusColor("encoding")
		// Use specific encoding percent if valid
		if enc := item.Encoding; enc != nil && enc.TotalFrames > 0 && enc.CurrentFrame > 0 {
			p := (float64(enc.CurrentFrame) / float64(enc.TotalFrames)) * 100
			if p > 0 {
				percent = p
			}
		}
	default:
		return // No active progress bar for other stages
	}

	bar := vm.drawProgressBar(percent, 30, color)
	fmt.Fprintf(b, "\n[%s::b]%s %s[::-]  %s %3.0f%%", color, icon, label, bar, percent)

	subLines := make([]string, 0, 2)
	if msg := strings.TrimSpace(item.Progress.Message); msg != "" {
		subLines = append(subLines, msg)
	}
	switch stage {
	case "encoding":
		if stats := formatEncodingMetrics(item.Encoding); stats != "" {
			subLines = append(subLines, stats)
		}
	case "ripping":
		if eta := vm.estimateETA(item); eta != "" {
			subLines = append(subLines, fmt.Sprintf("ETA: %s", eta))
		}
	}
	for i, line := range subLines {
		branch := "└─"
		if i < len(subLines)-1 {
			branch = "├─"
		}
		fmt.Fprintf(b, "\n[%s]%s[-] [%s]%s[-]", vm.theme.Text.Faint, branch, vm.theme.Text.Muted, tview.Escape(line))
	}
	fmt.Fprint(b, "\n")
}

func (vm *viewModel) drawProgressBar(percent float64, width int, color string) string {
	percent = clampPercent(percent)
	// Use Unicode blocks for smoother progress display
	// █ (full), ▓ (3/4), ▒ (1/2), ░ (1/4), and empty space
	blocks := []rune{'█', '▓', '▒', '░'}

	// Calculate how many full characters we need
	fullWidth := (percent / 100.0) * float64(width)
	fullChars := int(fullWidth)

	// Calculate the fractional part for the partial block
	fraction := fullWidth - float64(fullChars)

	var bar strings.Builder

	// Add full blocks
	if fullChars > 0 {
		fmt.Fprintf(&bar, "[%s]%s[-]", color, strings.Repeat(string(blocks[0]), fullChars))
	}

	// Add partial block based on fraction
	if fullChars < width && fraction > 0 {
		partialIdx := 3 - int(fraction*4) // Map 0-1 to 3-0 for decreasing density
		if partialIdx < 0 {
			partialIdx = 0
		}
		if partialIdx > 3 {
			partialIdx = 3
		}
		fmt.Fprintf(&bar, "[%s]%c[-]", vm.theme.Text.Muted, blocks[partialIdx])
		fullChars++
	}

	// Add empty space
	remaining := width - fullChars
	if remaining > 0 {
		fmt.Fprintf(&bar, "[%s]%s[-]", vm.theme.Text.Faint, strings.Repeat("░", remaining))
	}

	return bar.String()
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
	// Frame summary
	if enc.TotalFrames > 0 && enc.CurrentFrame > 0 {
		percent := int(math.Round((float64(enc.CurrentFrame) / float64(enc.TotalFrames)) * 100))
		parts = append(parts, fmt.Sprintf("%d/%d (%d%%)", enc.CurrentFrame, enc.TotalFrames, percent))
	}
	return strings.Join(parts, " • ")
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
