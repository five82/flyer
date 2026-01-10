package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

// detailContext represents the contextual state for detail view rendering.
type detailContext int

const (
	contextPending detailContext = iota
	contextActive
	contextCompleted
	contextFailed
)

// determineDetailContext returns the appropriate context for rendering the detail view.
func determineDetailContext(item spindle.QueueItem) detailContext {
	status := strings.ToLower(strings.TrimSpace(item.Status))

	if status == "failed" || item.NeedsReview || strings.TrimSpace(item.ErrorMessage) != "" {
		return contextFailed
	}
	if status == "completed" {
		return contextCompleted
	}
	if status == "pending" {
		return contextPending
	}
	return contextActive
}

func (vm *viewModel) updateDetail(row int) {
	if row <= 0 || row-1 >= len(vm.displayItems) {
		vm.detail.SetText(fmt.Sprintf("[%s]Select an item to view details[-]", vm.theme.Text.Faint))
		vm.detailState.lastID = 0
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

	// Always render: header, pipeline
	vm.renderDetailHeader(&b, item)
	fmt.Fprint(&b, "\n")
	vm.renderPipelineStatus(&b, item, totals)
	fmt.Fprint(&b, "\n")

	// Context-specific rendering
	switch determineDetailContext(item) {
	case contextFailed:
		vm.renderFailedContext(&b, item, episodes, mediaType)
	case contextCompleted:
		vm.renderCompletedContext(&b, item, titleLookup, episodeTitleIndex, episodes, totals, mediaType)
	case contextActive:
		vm.renderActiveContext(&b, item, summary, titleLookup, episodeTitleIndex, episodes, totals, mediaType)
	case contextPending:
		vm.renderPendingContext(&b, item)
	}

	content := b.String()
	prevRow, prevCol := vm.detail.GetScrollOffset()
	itemChanged := vm.detailState.lastID != item.ID
	vm.detail.SetText(content)
	vm.scrollDetailToActive(itemChanged, prevRow, prevCol)
	vm.detailState.lastID = item.ID
}

func (vm *viewModel) renderPipelineStatus(b *strings.Builder, item spindle.QueueItem, totals spindle.EpisodeTotals) {
	text := vm.theme.Text

	// Stages: Planned -> Identifying -> Ripped -> Encoded -> Subtitled -> Organizing -> Final
	stages := []struct {
		id    string
		label string
	}{
		{"planned", "Planned"},
		{"identifying", "Identifying"},
		{"ripped", "Ripped"},
		{"encoded", "Encoded"},
		{"subtitled", "Subtitled"},
		{"organizing", "Organizing"},
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

	// For TV episode items, derive identifying/subtitled/organizing by inspecting episode stages.
	identifyingCount := 0
	subtitledCount := 0
	organizingCount := 0
	if totals.Planned > 0 {
		episodes, _ := item.EpisodeSnapshot()
		for _, ep := range episodes {
			epStage := normalizeEpisodeStage(ep.Stage)
			// Identifying is complete if we've moved past it
			switch epStage {
			case "identified", "ripping", "ripped", "encoding", "encoded", "subtitling", "subtitled", "organizing", "final":
				identifyingCount++
			}
			// Subtitled includes both subtitled and final
			if epStage == "subtitled" || epStage == "final" {
				subtitledCount++
			}
			// Organizing is complete only when final
			if epStage == "final" {
				organizingCount++
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
			case "identifying":
				count = identifyingCount
			case "ripped":
				count = totals.Ripped
			case "encoded":
				count = totals.Encoded
			case "subtitled":
				count = subtitledCount
			case "organizing":
				count = organizingCount
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

func (vm *viewModel) scrollDetailToActive(itemChanged bool, prevRow, prevCol int) {
	// Basic scroll preservation or reset
	if itemChanged {
		vm.detail.ScrollTo(0, 0)
	} else {
		vm.detail.ScrollTo(prevRow, prevCol)
	}
}

// writeSection writes a section header with a title and divider line.
func (vm *viewModel) writeSection(b *strings.Builder, title string) {
	text := vm.theme.Text
	fmt.Fprintf(b, "\n[%s]%s[-]\n", text.Secondary, strings.ToUpper(title))
	fmt.Fprintf(b, "[%s]%s[-]\n", text.Faint, strings.Repeat("─", 38))
}

// renderDetailHeader renders the common header section (title, timestamps, chips).
func (vm *viewModel) renderDetailHeader(b *strings.Builder, item spindle.QueueItem) {
	text := vm.theme.Text
	now := time.Now()

	// Title
	title := composeTitle(item)
	fmt.Fprintf(b, "[%s]%s[-]\n", text.Heading, tview.Escape(title))

	// Timestamps
	formatStamp := func(ts time.Time) string {
		if ts.IsZero() {
			return ""
		}
		local := ts.In(time.Local)
		if local.Year() == now.Year() && local.YearDay() == now.YearDay() {
			return local.Format("15:04:05")
		}
		return local.Format("Jan 02 15:04")
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
	fmt.Fprintf(b, "%s\n", strings.Join(metaParts, fmt.Sprintf(" [%s]•[-] ", text.Faint)))

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
	fmt.Fprintf(b, "%s\n", strings.Join(chips, " "))
}

// renderActiveContext renders the detail view for items currently being processed.
func (vm *viewModel) renderActiveContext(b *strings.Builder, item spindle.QueueItem, summary spindle.RipSpecSummary, titleLookup map[int]*spindle.RipSpecTitleInfo, episodeTitleIndex map[string]int, episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals, mediaType string) {
	text := vm.theme.Text

	// Active progress bar
	vm.renderActiveProgress(b, item)

	// Specs section
	vm.writeSection(b, "Specs")
	vm.renderVideoSpecs(b, item)
	vm.renderAudioInfo(b, item)
	vm.renderEncodingConfig(b, item)
	vm.renderCropInfo(b, item)

	// Current stage
	currentStage := normalizeEpisodeStage(item.Progress.Stage)
	if currentStage == "" {
		currentStage = normalizeEpisodeStage(item.Status)
	}

	// Current episode (TV shows) or current selection (movies)
	if len(episodes) > 0 && mediaType != "movie" {
		activeIdx := vm.activeEpisodeIndex(item, episodes)
		if activeIdx >= 0 && activeIdx < len(episodes) {
			ep := episodes[activeIdx]
			vm.writeSection(b, "Current Episode")
			stage := vm.episodeStage(ep, currentStage, true)
			fmt.Fprint(b, vm.formatEpisodeFocusLine(ep, titleLookup, episodeTitleIndex, stage))
			b.WriteString("\n")
			if track := vm.describeEpisodeTrackInfo(&ep, titleLookup, episodeTitleIndex); track != "" {
				fmt.Fprintf(b, "[%s]Track:[-]   %s\n", text.Muted, track)
			}
			if files := vm.describeEpisodeFileStates(&ep); files != "" {
				fmt.Fprintf(b, "[%s]Files:[-]   %s\n", text.Muted, files)
			}
		}
	} else {
		if cut := vm.movieFocusLine(summary, currentStage); cut != "" {
			vm.writeSection(b, "Current Selection")
			fmt.Fprint(b, cut)
			b.WriteString("\n")
			if files := vm.describeItemFileStates(item); files != "" {
				fmt.Fprintf(b, "[%s]Files:[-]   %s\n", text.Muted, files)
			}
		}
	}

	// Episodes list (TV shows only)
	if len(episodes) > 0 && mediaType != "movie" {
		vm.renderEpisodesList(b, item, episodes, totals, currentStage, titleLookup, episodeTitleIndex)
	}
}

// renderCompletedContext renders the detail view for completed items.
func (vm *viewModel) renderCompletedContext(b *strings.Builder, item spindle.QueueItem, titleLookup map[int]*spindle.RipSpecTitleInfo, episodeTitleIndex map[string]int, episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals, mediaType string) {
	// Results section
	vm.writeSection(b, "Results")
	vm.renderSizeResult(b, item)
	vm.renderVideoSpecs(b, item)
	vm.renderAudioInfo(b, item)
	vm.renderEncodingConfig(b, item)
	vm.renderEncodeStats(b, item)
	vm.renderValidationSummary(b, item)

	// Subtitle summary (TV shows with multiple episodes) - inline in results
	if len(episodes) > 1 && mediaType != "movie" {
		vm.renderSubtitleSummary(b, item)
	}

	// Episodes list (TV shows only, collapsed by default)
	currentStage := normalizeEpisodeStage(item.Status)
	if len(episodes) > 0 && mediaType != "movie" {
		vm.renderEpisodesList(b, item, episodes, totals, currentStage, titleLookup, episodeTitleIndex)
	}
}

// renderFailedContext renders the detail view for failed or review items.
func (vm *viewModel) renderFailedContext(b *strings.Builder, item spindle.QueueItem, episodes []spindle.EpisodeStatus, mediaType string) {
	text := vm.theme.Text

	// Attention section (always show for failed)
	vm.writeSection(b, "Attention")
	if item.NeedsReview {
		reason := strings.TrimSpace(item.ReviewReason)
		if reason == "" {
			reason = "Needs operator review"
		}
		fmt.Fprintf(b, "[%s]Review:[-]   [%s]%s[-]\n", text.Warning, text.Primary, tview.Escape(reason))
	}
	if msg := strings.TrimSpace(item.ErrorMessage); msg != "" {
		fmt.Fprintf(b, "[%s]Error:[-]    [%s]%s[-]\n", text.Danger, text.Primary, tview.Escape(msg))
	}
	// Show detailed error info from Drapto if available
	if item.Encoding != nil && item.Encoding.Error != nil {
		err := item.Encoding.Error
		if title := strings.TrimSpace(err.Title); title != "" && title != strings.TrimSpace(item.ErrorMessage) {
			fmt.Fprintf(b, "[%s]Cause:[-]    [%s]%s[-]\n", text.Muted, text.Primary, tview.Escape(title))
		}
		if ctx := strings.TrimSpace(err.Context); ctx != "" {
			fmt.Fprintf(b, "[%s]Context:[-]  [%s]%s[-]\n", text.Muted, text.Secondary, tview.Escape(ctx))
		}
		if suggestion := strings.TrimSpace(err.Suggestion); suggestion != "" {
			fmt.Fprintf(b, "[%s]Suggest:[-]  [%s]%s[-]\n", text.Muted, vm.theme.StatusColor("completed"), tview.Escape(suggestion))
		}
	}

	// Last progress section
	vm.writeSection(b, "Last Progress")
	stage := normalizeEpisodeStage(item.Progress.Stage)
	if stage == "" {
		stage = normalizeEpisodeStage(item.Status)
	}
	if stage != "" && stage != "failed" {
		// Capitalize first letter
		stageDisplay := stage
		if len(stage) > 0 {
			stageDisplay = strings.ToUpper(stage[:1]) + stage[1:]
		}
		fmt.Fprintf(b, "[%s]Stage:[-]    [%s]%s[-]\n", text.Muted, text.Secondary, stageDisplay)
	}
	if item.Progress.Percent > 0 {
		fmt.Fprintf(b, "[%s]Progress:[-] [%s]%.0f%%[-]\n", text.Muted, text.Secondary, item.Progress.Percent)
	}

	// Show current episode if TV show
	if len(episodes) > 0 && mediaType != "movie" {
		activeIdx := vm.activeEpisodeIndex(item, episodes)
		if activeIdx >= 0 && activeIdx < len(episodes) {
			ep := episodes[activeIdx]
			label := formatEpisodeLabel(ep)
			title := strings.TrimSpace(ep.Title)
			if title == "" {
				title = strings.TrimSpace(ep.OutputBasename)
			}
			if title != "" {
				fmt.Fprintf(b, "[%s]Episode:[-]  [%s]%s[-] [%s]%s[-]\n", text.Muted, text.Secondary, label, text.Primary, tview.Escape(title))
			}
		}
	}

	// Validation details (show all steps if there are failures)
	vm.renderValidationDetails(b, item)

	// Paths section (always expanded for debugging)
	vm.writeSection(b, "Paths")
	vm.renderPathsExpanded(b, item)
}

// renderPendingContext renders the detail view for pending items.
func (vm *viewModel) renderPendingContext(b *strings.Builder, item spindle.QueueItem) {
	text := vm.theme.Text

	// Status section
	vm.writeSection(b, "Status")
	fmt.Fprintf(b, "[%s]Waiting in queue...[-]\n", text.Muted)

	// Metadata section (if available)
	if metaRows := summarizeMetadata(item.Metadata); len(metaRows) > 0 {
		vm.writeSection(b, "Metadata")
		b.WriteString(vm.formatMetadata(metaRows))
	}
}

// renderValidationSummary renders a one-line validation summary for completed items.
func (vm *viewModel) renderValidationSummary(b *strings.Builder, item spindle.QueueItem) {
	if item.Encoding == nil || item.Encoding.Validation == nil {
		return
	}
	text := vm.theme.Text
	v := item.Encoding.Validation
	total := len(v.Steps)
	if total == 0 {
		return
	}
	passed := 0
	for _, step := range v.Steps {
		if step.Passed {
			passed++
		}
	}
	if v.Passed {
		fmt.Fprintf(b, "[%s]Validation:[-] [%s]✓ Passed[-] [%s](%d/%d checks)[-]\n",
			text.Muted, vm.theme.StatusColor("completed"), text.Faint, passed, total)
	} else {
		fmt.Fprintf(b, "[%s]Validation:[-] [%s]✗ Failed[-] [%s](%d/%d checks)[-]\n",
			text.Muted, text.Danger, text.Faint, passed, total)
	}
}

// renderValidationDetails renders detailed validation step results for failed items.
func (vm *viewModel) renderValidationDetails(b *strings.Builder, item spindle.QueueItem) {
	if item.Encoding == nil || item.Encoding.Validation == nil {
		return
	}
	v := item.Encoding.Validation
	if len(v.Steps) == 0 {
		return
	}
	// Only show details if validation failed or if there are any failed steps
	hasFailures := !v.Passed
	if !hasFailures {
		for _, step := range v.Steps {
			if !step.Passed {
				hasFailures = true
				break
			}
		}
	}
	if !hasFailures {
		return
	}

	text := vm.theme.Text
	vm.writeSection(b, "Validation")

	for _, step := range v.Steps {
		icon := "✓"
		color := vm.theme.StatusColor("completed")
		if !step.Passed {
			icon = "✗"
			color = text.Danger
		}
		name := strings.TrimSpace(step.Name)
		if name == "" {
			name = "Check"
		}
		fmt.Fprintf(b, "[%s]%s[-] [%s]%s[-]", color, icon, text.Secondary, name)
		if details := strings.TrimSpace(step.Details); details != "" {
			fmt.Fprintf(b, " [%s]%s[-]", text.Faint, tview.Escape(details))
		}
		b.WriteString("\n")
	}
}
