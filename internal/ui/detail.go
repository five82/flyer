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

	// -- HEADER --
	title := composeTitle(item)
	fmt.Fprintf(&b, "[%s]%s[-]\n", text.Heading, tview.Escape(title))

	// Status & Chips
	status := vm.statusChip(item.Status)
	lane := vm.laneChip(determineLane(item.Status))
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

	// -- EPISODES LIST --
	if len(episodes) > 0 && mediaType != "movie" {
		writeSection("Episodes")
		if !item.EpisodesSynced {
			fmt.Fprintf(&b, "[%s]Episode numbers not confirmed[-]\n", vm.theme.Text.Warning)
		}

		collapsed := vm.episodesCollapsed(item.ID)
		if collapsed {
			summary := vm.describeEpisodeTotals(episodes, totals)
			fmt.Fprintf(&b, "[%s]%s[-]  [%s](press t to expand)[-]\n", text.Primary, summary, text.Faint)
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

	// Stages: Planned -> Ripped -> Encoded -> Final
	stages := []struct {
		id    string
		label string
	}{
		{"planned", "Planned"},
		{"ripped", "Ripped"},
		{"encoded", "Encoded"},
		{"final", "Final"},
	}

	// Identify current active stage loosely for movies
	activeStage := normalizeEpisodeStage(item.Progress.Stage)
	if activeStage == "" {
		activeStage = normalizeEpisodeStage(item.Status)
	}

	for i, stage := range stages {
		if i > 0 {
			fmt.Fprintf(b, " [%s]→[-] ", text.Faint)
		}

		// Determine state
		isComplete := false
		isCurrent := false

		// For TV shows, use totals
		if totals.Planned > 0 {
			// Logic:
			// Planned: Always done if we have episodes
			// Ripped: if Ripped == Planned
			// Encoded: if Encoded == Planned
			// Final: if Final == Planned
			switch stage.id {
			case "planned":
				isComplete = true // implied
			case "ripped":
				isComplete = totals.Ripped >= totals.Planned
			case "encoded":
				isComplete = totals.Encoded >= totals.Planned
			case "final":
				isComplete = totals.Final >= totals.Planned
			}
			// Current if not complete but previous is complete?
			// Simplification: just show counts for TV
		} else {
			// For Movies/Single items
			// Logic depending on item.Status
			// e.g. if item.Status == "encoding", then Planned, Ripped are complete. Encoded is current.
			// Mapping is approximate since status strings vary.
			s := scoreStage(activeStage)
			myS := scoreStage(stage.id)
			if s > myS {
				isComplete = true
			} else if s == myS {
				isCurrent = true
			}
		}

		color := text.Muted
		icon := "○"

		if isComplete {
			color = vm.theme.StatusColor("completed")
			icon = "●"
		} else if isCurrent {
			color = vm.theme.StatusColor("processing") // or accent
			icon = "◉"
		}

		if totals.Planned > 0 {
			// TV Show: Show counts
			count := totals.Planned
			switch stage.id {
			case "ripped":
				count = totals.Ripped
			case "encoded":
				count = totals.Encoded
			case "final":
				count = totals.Final
			}
			// For planned, it's just totals.Planned

			labelColor := text.Secondary
			if count == totals.Planned {
				labelColor = vm.theme.StatusColor("completed")
			} else if count > 0 {
				labelColor = vm.theme.StatusColor("processing")
			}

			fmt.Fprintf(b, "[%s]%s[-] [%s]%d[-]", labelColor, stage.label, text.Muted, count)
		} else {
			// Single Item: Show Pipeline
			fmt.Fprintf(b, "[%s]%s %s[-]", color, icon, stage.label)
		}
	}
}

func scoreStage(stage string) int {
	switch stage {
	case "final", "completed", "success":
		return 4
	case "encoded", "encoding", "organizing":
		return 3
	case "ripped", "ripping":
		return 2
	case "planned", "identifying", "identified", "pending":
		return 1
	}
	return 0
}

// -- Helpers moved and adapted from navigation.go --

func normalizeEpisodeStage(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
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
	if s == "final" || s == "completed" || s == "organizing" {
		return "final"
	}
	return "planned"
}

func (vm *viewModel) activeEpisodeIndex(item spindle.QueueItem, episodes []spindle.EpisodeStatus) int {
	if len(episodes) == 0 {
		return -1
	}

	// 1. Precise Match: File path
	// If we are ripping, item.RippedFile should match episode.RippedPath (or basename)
	// If we are encoding, item.EncodedFile should match episode.EncodedPath
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
	if stage == "encoding" {
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
	case "encoded":
		color = vm.theme.StatusColor("encoded")
		label = "ENCD"
	case "ripped":
		color = vm.theme.StatusColor("ripping")
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

func (vm *viewModel) logPreview(path string) string {
	// Stub for now or needs to be properly linked if we want that feature back.
	// For this refactor, I'm omitting the expensive log preview from the details view
	// unless specifically requested, to keep it clean.
	return ""
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
	if lang := strings.TrimSpace(ep.SubtitleLanguage); lang != "" {
		extra = append(extra, strings.ToUpper(lang))
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
	color := vm.theme.StatusColor("processing")

	if stage == "ripping" {
		label = "RIPPING"
		color = vm.theme.StatusColor("ripping")
	} else if stage == "encoding" {
		label = "ENCODING"
		color = vm.theme.StatusColor("encoding")
		// Use specific encoding percent if valid
		if enc := item.Encoding; enc != nil && enc.TotalFrames > 0 && enc.CurrentFrame > 0 {
			p := (float64(enc.CurrentFrame) / float64(enc.TotalFrames)) * 100
			if p > 0 {
				percent = p
			}
		}
	} else {
		return // No active progress bar for other stages
	}

	bar := vm.drawProgressBar(percent, 30, color)
	fmt.Fprintf(b, "\n[%s::b]%s[::-]  %s %3.0f%%", color, label, bar, percent)

	// Add stats line
	if stage == "encoding" {
		if stats := formatEncodingMetrics(item.Encoding); stats != "" {
			fmt.Fprintf(b, "   [%s]%s[-]", vm.theme.Text.Muted, stats)
		}
	} else if stage == "ripping" {
		// Maybe add ETA if available?
		if eta := vm.estimateETA(item); eta != "" {
			fmt.Fprintf(b, "   [%s]%s[-]", vm.theme.Text.Faint, eta)
		}
	}
	fmt.Fprint(b, "\n")
}

func (vm *viewModel) drawProgressBar(percent float64, width int, color string) string {
	percent = clampPercent(percent)
	full := int(math.Round(percent / 100 * float64(width)))
	if full < 0 {
		full = 0
	}
	if full > width {
		full = width
	}
	empty := width - full
	return fmt.Sprintf("[%s]%s[%s]%s[-]", color, strings.Repeat("━", full), "gray", strings.Repeat("━", empty))
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
