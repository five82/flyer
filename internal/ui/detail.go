package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"

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

// initDetailViewport initializes the detail viewport.
func (m *Model) initDetailViewport() {
	m.detailViewport = viewport.New(
		viewport.WithWidth(m.width-4),
		viewport.WithHeight(m.height-6),
	)
	m.detailViewport.Style = lipgloss.NewStyle()
}

// updateDetailViewport updates the detail viewport content.
func (m *Model) updateDetailViewport() {
	if m.detailViewport.Width() == 0 {
		m.initDetailViewport()
	}

	// Compute split-pane dimensions matching renderQueue layout
	contentHeight := m.height - 2 // header + cmdbar
	var tableWidth int
	if m.width >= 160 {
		tableWidth = m.width * 30 / 100
	} else {
		tableWidth = m.width * 40 / 100
	}
	detailWidth := m.width - tableWidth

	m.detailViewport.SetWidth(detailWidth - 2)    // minus box side borders
	m.detailViewport.SetHeight(contentHeight - 2) // minus box top/bottom borders

	// Use focus-aware background
	bgColor := m.theme.SurfaceAlt
	if m.focusedPane == 1 {
		bgColor = m.theme.FocusBg
	}

	// Viewport style covers empty lines below content
	m.detailViewport.Style = lipgloss.NewStyle().Background(lipgloss.Color(bgColor))

	// Get selected item
	item := m.getSelectedItem()
	if item == nil {
		bg := NewBgStyle(bgColor)
		m.detailViewport.SetContent(bg.FillLine(m.theme.Styles().MutedText.Render("Select an item to view details"), m.detailViewport.Width()))
		return
	}

	// Render detail content and fill each line to viewport width
	content := m.renderDetailContent(*item, detailWidth-4, bgColor)
	bg := NewBgStyle(bgColor)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = bg.FillLine(line, m.detailViewport.Width())
	}
	m.detailViewport.SetContent(strings.Join(lines, "\n"))
}

// getSelectedItem returns the currently selected queue item.
func (m *Model) getSelectedItem() *spindle.QueueItem {
	items := m.getSortedItems()
	if m.selectedRow < 0 || m.selectedRow >= len(items) {
		return nil
	}
	return &items[m.selectedRow]
}

// renderDetailContent renders the full detail content for an item.
// The width parameter controls text wrapping (0 means use default m.width).
// The bgColor parameter sets the background color for all styled text.
func (m *Model) renderDetailContent(item spindle.QueueItem, width int, bgColor string) string {
	if width <= 0 {
		width = m.width - 4
	}
	_ = width // TODO: use for text wrapping
	var b strings.Builder
	styles := m.theme.Styles().WithBackground(bgColor)
	bg := NewBgStyle(bgColor)

	// Header section
	m.renderDetailHeader(&b, item, styles, bg)

	// Pipeline status: one row per scheduler task, in pipeline order, with
	// inline progress for whatever is currently running (including
	// concurrent branches during rip/encode overlap).
	m.writeSection(&b, "Pipeline", styles, bg)
	m.renderTaskBoard(&b, item, styles, bg)

	// Context-specific content
	ctx := determineDetailContext(item)
	switch ctx {
	case contextPending:
		m.renderPendingDetail(&b, item, styles, bg)
	case contextActive:
		m.renderActiveDetail(&b, item, styles, bg)
	case contextCompleted:
		m.renderCompletedDetail(&b, item, styles, bg)
	case contextFailed:
		m.renderFailedDetail(&b, item, styles, bg)
	}

	return b.String()
}

// renderDetailHeader renders the item header (title, timestamps, chips).
func (m *Model) renderDetailHeader(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	now := time.Now()

	title := composeTitle(item)
	b.WriteString(bg.Render(title, styles.Text.Bold(true)))
	b.WriteString("\n")

	// Timestamps and ID
	metaParts := []string{
		bg.Render(fmt.Sprintf("#%d", item.ID), styles.MutedText),
	}

	if created := parseTimestamp(item.CreatedAt); !created.IsZero() {
		metaParts = append(metaParts,
			bg.Render("created", styles.FaintText)+bg.Space()+
				bg.Render(formatTimestamp(created, now), styles.Text))
	}

	if updated := parseTimestamp(item.UpdatedAt); !updated.IsZero() {
		ago := now.Sub(updated)
		metaParts = append(metaParts,
			bg.Render("updated", styles.FaintText)+bg.Space()+
				bg.Render(formatTimestamp(updated, now), styles.Text)+bg.Space()+
				bg.Render("("+humanizeDuration(ago)+")", styles.MutedText))
	}

	b.WriteString(bg.Join(metaParts, " • "))
	b.WriteString("\n")

	// Status chips
	chips := m.renderStatusChips(item, styles, bg)
	b.WriteString(chips)
	b.WriteString("\n")
}

// renderStatusChips renders the status badges for an item.
func (m *Model) renderStatusChips(item spindle.QueueItem, styles Styles, bg BgStyle) string {
	var chips []string

	// Status chip: role-colored text from the task/stage registry, so an
	// unrecognized stage name renders neutrally instead of crashing or
	// falling back to a hardcoded color table.
	info := stageDisplay(itemDisplayStage(item))
	label := info.label
	if item.IsTerminal() {
		label = info.doneLabel
	}
	chips = append(chips, roleStyle(info.role, styles).Bold(true).Render(strings.ToUpper(label)))

	// Media type chip
	mediaType := detectMediaType(item.Metadata)
	if mediaType != "" {
		label := "MOVIE"
		if mediaType == "tv" {
			label = "TV"
		}
		mediaChip := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.Accent)).
			Padding(0, 1).
			Render(label)
		chips = append(chips, mediaChip)
	}

	// Review badge
	if item.NeedsReview {
		reviewChip := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.Warning)).
			Padding(0, 1).
			Render("REVIEW")
		chips = append(chips, reviewChip)
	}

	// Error badge
	if strings.TrimSpace(item.ErrorMessage) != "" {
		errorChip := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.Danger)).
			Padding(0, 1).
			Render("ERROR")
		chips = append(chips, errorChip)
	}

	// CACHE badge (rip cache hit, reported via the ripping task's message)
	if isRipCacheHit(item) {
		cacheChip := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.Info)).
			Padding(0, 1).
			Render("CACHE")
		chips = append(chips, cacheChip)
	}

	return strings.Join(chips, bg.Space())
}

// isRipCacheHit reports whether any task's progress message indicates a rip
// cache hit.
func isRipCacheHit(item spindle.QueueItem) bool {
	for _, t := range item.Tasks {
		if strings.Contains(strings.ToLower(t.Progress.Message), "rip cache hit") {
			return true
		}
	}
	return false
}

// writeSection writes a section header.
func (m *Model) writeSection(b *strings.Builder, title string, styles Styles, bg BgStyle) {
	b.WriteString("\n")
	b.WriteString(bg.Render(titleCase(title), styles.MutedText.Bold(true)))
	b.WriteString("\n")
	b.WriteString(bg.Render(strings.Repeat("─", 24), styles.FaintText))
	b.WriteString("\n")
}

func renderDetailField(b *strings.Builder, bg BgStyle, label string, labelStyle lipgloss.Style, value string, valueStyle lipgloss.Style) {
	if strings.TrimSpace(value) == "" {
		return
	}
	b.WriteString(bg.Render(fmt.Sprintf("%-8s", label), labelStyle))
	b.WriteString(bg.Render(value, valueStyle))
	b.WriteString("\n")
}

// determineDetailContext returns the appropriate context for rendering.
func determineDetailContext(item spindle.QueueItem) detailContext {
	status := strings.ToLower(strings.TrimSpace(item.Stage))

	if status == "failed" || item.NeedsReview || strings.TrimSpace(item.ErrorMessage) != "" {
		return contextFailed
	}
	if status == "completed" {
		return contextCompleted
	}
	if status == "identification" && !item.InProgress {
		return contextPending
	}
	return contextActive
}

// renderPendingDetail renders detail for pending items.
func (m *Model) renderPendingDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	m.writeSection(b, "Status", styles, bg)
	renderDetailField(b, bg, "State", styles.MutedText, "Waiting in queue...", styles.MutedText)

	if detectMediaType(item.Metadata) == "movie" {
		m.renderMovieScope(b, item, styles, bg)
	}

	// Metadata section (if available)
	if metaRows := summarizeMetadata(item.Metadata); len(metaRows) > 0 {
		m.writeSection(b, "Metadata", styles, bg)
		m.renderMetadata(b, metaRows, styles, bg)
	}
}

// renderActiveDetail renders detail for active/processing items: one
// section per RUNNING task, so overlap windows show every live branch's
// context at once (the right details at the right time).
func (m *Model) renderActiveDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	episodes, totals := item.EpisodeSnapshot()
	mediaType := detectMediaType(item.Metadata)

	running := item.RunningTasks()
	for _, task := range running {
		m.renderTaskSection(b, item, task, episodes, styles, bg)
	}
	if len(running) == 0 {
		m.writeSection(b, "Status", styles, bg)
		renderDetailField(b, bg, "State", styles.MutedText, "Waiting for scheduler...", styles.MutedText)
	}

	if mediaType == "movie" {
		m.renderMovieScope(b, item, styles, bg)
	}

	m.renderEpisodeList(b, item, styles, bg, totals)
}

// renderTaskSection renders one running task's working context. The task
// board above already shows the bar/percent/message; this section adds the
// data that matters for that task type.
func (m *Model) renderTaskSection(b *strings.Builder, item spindle.QueueItem, task spindle.Task, episodes []spindle.EpisodeStatus, styles Styles, bg BgStyle) {
	info := stageDisplay(task.Type)
	m.writeSection(b, info.label, styles, bg)

	// The episode this task reports working on, when it names one.
	if key := strings.ToLower(strings.TrimSpace(task.ActiveAssetKey)); key != "" {
		for i := range episodes {
			if strings.ToLower(episodes[i].Key) != key {
				continue
			}
			ep := episodes[i]
			renderDetailField(b, bg, "Episode", styles.MutedText, strings.TrimSpace(formatEpisodeLabel(ep)+" "+episodeDisplayTitle(ep)), styles.Text)
			if track := describeEpisodeTrackInfo(&ep); track != "" {
				renderDetailField(b, bg, "Track", styles.MutedText, track, styles.Text)
			}
			break
		}
	}

	switch task.Type {
	case "encoding":
		m.renderEstimatedSize(b, item, styles, bg)
		m.renderVideoSpecs(b, item, styles, bg)
		m.renderAudioInfo(b, item, styles, bg)
		m.renderEncodingConfig(b, item, styles, bg)
		m.renderCropInfo(b, item, styles, bg)
	case "episode_identification":
		if cid := item.ContentID; cid != nil {
			renderDetailField(b, bg, "Method", styles.MutedText, cid.Method, styles.Text)
			if cid.TranscribedEpisodes > 0 || cid.MatchedEpisodes > 0 {
				renderDetailField(b, bg, "Matched", styles.MutedText,
					fmt.Sprintf("%d matched · %d unresolved · %d low confidence",
						cid.MatchedEpisodes, cid.UnresolvedEpisodes, cid.LowConfidenceCount), styles.Text)
			}
		}
	case "analysis":
		m.renderAudioInfo(b, item, styles, bg)
		if item.CommentaryCount > 0 {
			renderDetailField(b, bg, "Comment.", styles.MutedText,
				fmt.Sprintf("%d commentary track(s) detected", item.CommentaryCount), styles.Text)
		}
	case "subtitling":
		done := 0
		for _, ep := range episodes {
			if strings.TrimSpace(ep.SubtitleSource) != "" {
				done++
			}
		}
		if done > 0 {
			renderDetailField(b, bg, "Subs", styles.MutedText,
				fmt.Sprintf("%d of %d generated", done, max(len(episodes), 1)), styles.Text)
		}
	case "organizing":
		if files := m.describeItemFileStates(item); files != "" {
			renderDetailField(b, bg, "Files", styles.MutedText, files, styles.Text)
		}
	}
}

// renderRecoverySummary renders leftover file state for a stopped/failed
// item. Stage, attempts, and error are already visible in the Pipeline task
// board above -- this section only adds what that board doesn't show.
func (m *Model) renderRecoverySummary(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	files := m.describeItemFileStates(item)
	if files == "" {
		return
	}
	m.writeSection(b, "Recovery", styles, bg)
	renderDetailField(b, bg, "Files", styles.MutedText, files, styles.Text)
}

func (m *Model) renderCompletionAudit(b *strings.Builder, item spindle.QueueItem, episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals, mediaType string, styles Styles, bg BgStyle) {
	m.writeSection(b, "Audit", styles, bg)
	if mediaType == "movie" {
		if files := m.describeItemFileStates(item); files != "" {
			renderDetailField(b, bg, "Files", styles.MutedText, files, styles.Text)
		}
		m.renderValidationSummary(b, item, styles, bg)
		m.renderSubtitleInfo(b, item, styles, bg)
		return
	}
	if len(episodes) == 0 {
		return
	}
	renderDetailField(b, bg, "Batch", styles.MutedText, fmt.Sprintf("%d planned · %d matched · %d final", totals.Planned, matchedEpisodeCount(item, episodes), totals.Final), styles.Text)
	if totals.Final < totals.Planned {
		renderDetailField(b, bg, "Missing", styles.WarningText, fmt.Sprintf("%d final outputs", totals.Planned-totals.Final), styles.Text)
	}
	m.renderSubtitleSummary(b, item, styles, bg)
}

func isHealthyCompletedTV(item spindle.QueueItem, episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals) bool {
	if len(episodes) <= 1 {
		return false
	}
	if detectMediaType(item.Metadata) != "tv" {
		return false
	}
	if item.NeedsReview || strings.TrimSpace(item.ErrorMessage) != "" {
		return false
	}
	if totals.Planned == 0 || totals.Final != totals.Planned {
		return false
	}
	if matchedEpisodeCount(item, episodes) != totals.Planned {
		return false
	}
	if len(spindle.FilterFailed(episodes)) > 0 {
		return false
	}
	if item.Encoding != nil && item.Encoding.Validation != nil && !item.Encoding.Validation.Passed {
		return false
	}
	return true
}

func (m *Model) renderCompletedTVSummary(b *strings.Builder, item spindle.QueueItem, episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals, styles Styles, bg BgStyle) {
	m.writeSection(b, "Summary", styles, bg)
	renderDetailField(b, bg, "Batch", styles.MutedText, fmt.Sprintf("%d planned · %d matched · %d final", totals.Planned, matchedEpisodeCount(item, episodes), totals.Final), styles.Text)
	m.renderValidationSummary(b, item, styles, bg)
	m.renderSubtitleSummary(b, item, styles, bg)
	m.renderSizeResult(b, item, styles, bg)
	m.renderEncodeStats(b, item, styles, bg)
	m.renderVideoSpecs(b, item, styles, bg)
	m.renderAudioInfo(b, item, styles, bg)
	m.renderEncodingConfig(b, item, styles, bg)
}

// renderCompletedDetail renders detail for completed items. No Focus
// section: nothing is active, and Summary/Results carry the outcome.
func (m *Model) renderCompletedDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	episodes, totals := item.EpisodeSnapshot()
	mediaType := detectMediaType(item.Metadata)

	if isHealthyCompletedTV(item, episodes, totals) {
		m.renderCompletedTVSummary(b, item, episodes, totals, styles, bg)
		m.renderEpisodeList(b, item, styles, bg, totals)
		return
	}

	m.writeSection(b, "Results", styles, bg)
	m.renderSizeResult(b, item, styles, bg)
	m.renderVideoSpecs(b, item, styles, bg)
	m.renderAudioInfo(b, item, styles, bg)
	m.renderEncodingConfig(b, item, styles, bg)
	m.renderEncodeStats(b, item, styles, bg)
	m.renderValidationSummary(b, item, styles, bg)
	if len(episodes) > 1 && mediaType != "movie" {
		m.renderSubtitleSummary(b, item, styles, bg)
	} else {
		m.renderSubtitleInfo(b, item, styles, bg)
	}

	if mediaType == "movie" {
		m.renderMovieScope(b, item, styles, bg)
	}
	m.renderCompletionAudit(b, item, episodes, totals, mediaType, styles, bg)
	m.renderEpisodeList(b, item, styles, bg, totals)
}

// renderFailedDetail renders detail for failed/review items.
func (m *Model) renderFailedDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	episodes, totals := item.EpisodeSnapshot()
	mediaType := detectMediaType(item.Metadata)

	m.renderDetailFocus(b, item, episodes, styles, bg)

	// Attention section (always show for failed)
	m.writeSection(b, "Attention", styles, bg)

	// Review reason(s)
	if item.NeedsReview {
		reason := strings.Join(item.ReviewReasons, "; ")
		if reason == "" {
			reason = "Needs operator review"
		}
		renderDetailField(b, bg, "Review", styles.WarningText, reason, styles.Text)
	}

	// Error message
	if msg := strings.TrimSpace(item.ErrorMessage); msg != "" {
		renderDetailField(b, bg, "Error", styles.DangerText, msg, styles.Text)
	}

	// Detailed error from Reel
	if item.Encoding != nil && item.Encoding.Error != nil {
		err := item.Encoding.Error
		if title := strings.TrimSpace(err.Title); title != "" && title != strings.TrimSpace(item.ErrorMessage) {
			renderDetailField(b, bg, "Cause", styles.MutedText, title, styles.Text)
		}
		if ctx := strings.TrimSpace(err.Context); ctx != "" {
			renderDetailField(b, bg, "Context", styles.MutedText, ctx, styles.Text)
		}
		if suggestion := strings.TrimSpace(err.Suggestion); suggestion != "" {
			renderDetailField(b, bg, "Suggest", styles.MutedText, suggestion, styles.SuccessText)
		}
	}

	m.renderRecoverySummary(b, item, styles, bg)
	if mediaType == "movie" {
		m.renderMovieScope(b, item, styles, bg)
	} else {
		m.renderEpisodeList(b, item, styles, bg, totals)
	}

	// Validation details (show all steps if there are failures)
	m.renderValidationDetails(b, item, styles, bg)
}

// renderDetailFocus renders the "Focus" section: the single most relevant
// episode or movie-level line for what's happening right now. Renders
// nothing (no header, no placeholder) when there is nothing to focus on.
func (m *Model) renderDetailFocus(b *strings.Builder, item spindle.QueueItem, episodes []spindle.EpisodeStatus, styles Styles, bg BgStyle) {
	mediaType := detectMediaType(item.Metadata)
	if len(episodes) > 0 && mediaType != "movie" {
		idx, ok := activeEpisodeIndex(item, episodes)
		if !ok {
			return
		}
		ep := episodes[idx]
		m.writeSection(b, "Focus", styles, bg)
		m.renderEpisodeFocusLine(b, ep, styles, bg)
		b.WriteString("\n")
		renderDetailField(b, bg, "Episode", styles.MutedText, formatEpisodeLabel(ep), styles.Text)
		if track := describeEpisodeTrackInfo(&ep); track != "" {
			renderDetailField(b, bg, "Track", styles.MutedText, track, styles.Text)
		}
		if files := m.describeEpisodeFileStates(&ep); files != "" {
			renderDetailField(b, bg, "Files", styles.MutedText, files, styles.Text)
		}
		if issue := describeEpisodeIssue(ep); issue != "" {
			renderDetailField(b, bg, "Issue", styles.MutedText, issue, styles.WarningText)
		}
		return
	}
	m.writeSection(b, "Focus", styles, bg)
	m.renderMovieFocus(b, item, styles, bg)
}

// renderMovieFocus renders the focus line for a movie item: current
// activity (from the stage/task registry) plus its source title.
func (m *Model) renderMovieFocus(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	info := stageDisplay(itemDisplayStage(item))
	label := info.label
	if item.IsTerminal() {
		label = info.doneLabel
	}
	b.WriteString(bg.Render(strings.ToUpper(label), roleStyle(info.role, styles).Bold(true)))
	b.WriteString("\n")

	if value := sourceSummary(item.Source); value != "" {
		renderDetailField(b, bg, "Source", styles.MutedText, value, styles.Text)
	}
	if files := m.describeItemFileStates(item); files != "" {
		renderDetailField(b, bg, "Files", styles.MutedText, files, styles.Text)
	}
}

// renderMovieScope renders the "Scope" section for a movie item: source
// title and, while the item is still in flight, subtitle/validation status.
func (m *Model) renderMovieScope(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	if detectMediaType(item.Metadata) != "movie" {
		return
	}

	m.writeSection(b, "Scope", styles, bg)
	if value := sourceSummary(item.Source); value != "" {
		renderDetailField(b, bg, "Source", styles.MutedText, value, styles.Text)
	}
	if files := m.describeItemFileStates(item); files != "" {
		renderDetailField(b, bg, "Files", styles.MutedText, files, styles.Text)
	}
	if !strings.EqualFold(item.Stage, "completed") {
		m.renderSubtitleInfo(b, item, styles, bg)
		m.renderValidationSummary(b, item, styles, bg)
	}
}

// sourceSummary formats a movie's primary source title, e.g.
// "Title 02 (118m)". Returns "" when no source info is available.
func sourceSummary(src *spindle.SourceTitle) string {
	if src == nil {
		return ""
	}
	value := strings.TrimSpace(src.Name)
	if value == "" && src.TitleID > 0 {
		value = fmt.Sprintf("Title %02d", src.TitleID)
	}
	if value == "" {
		return ""
	}
	if src.DurationSeconds > 0 {
		value += " (" + formatRuntime(src.DurationSeconds) + ")"
	}
	return value
}
