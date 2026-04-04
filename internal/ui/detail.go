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

	// Pipeline status
	m.writeSection(&b, "Pipeline", styles, bg)
	m.renderPipelineStatus(&b, item, styles, bg)

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
	chips := m.renderStatusChips(item, bg)
	b.WriteString(chips)
	b.WriteString("\n")
}

// renderStatusChips renders the status badges for an item.
func (m *Model) renderStatusChips(item spindle.QueueItem, bg BgStyle) string {
	var chips []string

	// Status chip
	colorHex := m.theme.StatusColors[strings.ToLower(item.Stage)]
	if colorHex == "" {
		colorHex = m.theme.Muted
	}
	statusColor := lipgloss.Color(colorHex)
	statusChip := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Background)).
		Background(statusColor).
		Padding(0, 1).
		Render(strings.ToUpper(titleCase(item.Stage)))
	chips = append(chips, statusChip)

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

	// CACHE badge (rip cache hit)
	if isRipCacheHitMessage(item.Progress.Message) {
		cacheChip := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.Info)).
			Padding(0, 1).
			Render("CACHE")
		chips = append(chips, cacheChip)
	}

	return strings.Join(chips, bg.Space())
}

// isRipCacheHitMessage checks if the progress message indicates a rip cache hit.
func isRipCacheHitMessage(message string) bool {
	msg := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(msg, "rip cache hit")
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
	summary, _ := item.ParseRipSpec()
	m.writeSection(b, "Status", styles, bg)
	renderDetailField(b, bg, "State", styles.MutedText, "Waiting in queue...", styles.MutedText)

	if detectMediaType(item.Metadata) == "movie" {
		m.renderMovieScope(b, item, summary, styles, bg)
	}

	// Metadata section (if available)
	if metaRows := summarizeMetadata(item.Metadata); len(metaRows) > 0 {
		m.writeSection(b, "Metadata", styles, bg)
		m.renderMetadata(b, metaRows, styles, bg)
	}
}

// renderActiveDetail renders detail for active/processing items.
func (m *Model) renderActiveDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	summary, _ := item.ParseRipSpec()
	titleLookup, episodeTitleIndex := buildTitleLookups(summary)
	episodes, totals := item.EpisodeSnapshot()
	mediaType := detectMediaType(item.Metadata)

	currentStage := itemCurrentStage(item)
	m.renderDetailFocus(b, item, summary, titleLookup, episodeTitleIndex, episodes, currentStage, styles, bg)

	m.writeSection(b, "Activity", styles, bg)
	m.renderActiveProgress(b, item, styles, bg)
	if msg := strings.TrimSpace(item.Progress.Message); msg != "" {
		renderDetailField(b, bg, "State", styles.MutedText, msg, styles.Text)
	}
	m.renderEstimatedSize(b, item, styles, bg)

	m.writeSection(b, "Details", styles, bg)
	m.renderVideoSpecs(b, item, styles, bg)
	m.renderAudioInfo(b, item, styles, bg)
	m.renderEncodingConfig(b, item, styles, bg)
	m.renderCropInfo(b, item, styles, bg)

	if mediaType == "movie" {
		m.renderMovieScope(b, item, summary, styles, bg)
	}

	m.renderEpisodeList(b, item, styles, bg, titleLookup, episodeTitleIndex, currentStage, totals)
}

func (m *Model) renderRecoverySummary(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	files := m.describeItemFileStates(item)
	stage := itemCurrentStage(item)
	if files == "" && (stage == "" || stage == "failed") && item.Progress.Percent <= 0 {
		return
	}
	m.writeSection(b, "Recovery", styles, bg)
	if stage != "" && stage != "failed" {
		renderDetailField(b, bg, "Stopped", styles.MutedText, titleCase(stage), styles.Text)
	}
	if item.Progress.Percent > 0 {
		renderDetailField(b, bg, "Progress", styles.MutedText, fmt.Sprintf("%.0f%%", item.Progress.Percent), styles.Text)
	}
	if files != "" {
		renderDetailField(b, bg, "Files", styles.MutedText, files, styles.Text)
	}
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

// renderCompletedDetail renders detail for completed items.
func (m *Model) renderCompletedDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	summary, _ := item.ParseRipSpec()
	titleLookup, episodeTitleIndex := buildTitleLookups(summary)
	episodes, totals := item.EpisodeSnapshot()
	mediaType := detectMediaType(item.Metadata)

	currentStage := normalizeEpisodeStage(item.Stage)
	m.renderDetailFocus(b, item, summary, titleLookup, episodeTitleIndex, episodes, currentStage, styles, bg)

	if isHealthyCompletedTV(item, episodes, totals) {
		m.renderCompletedTVSummary(b, item, episodes, totals, styles, bg)
		m.renderEpisodeList(b, item, styles, bg, titleLookup, episodeTitleIndex, currentStage, totals)
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
		m.renderMovieScope(b, item, summary, styles, bg)
	}
	m.renderCompletionAudit(b, item, episodes, totals, mediaType, styles, bg)
	m.renderEpisodeList(b, item, styles, bg, titleLookup, episodeTitleIndex, currentStage, totals)
}

// renderFailedDetail renders detail for failed/review items.
func (m *Model) renderFailedDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	summary, _ := item.ParseRipSpec()
	titleLookup, episodeTitleIndex := buildTitleLookups(summary)
	episodes, totals := item.EpisodeSnapshot()
	mediaType := detectMediaType(item.Metadata)

	currentStage := itemCurrentStage(item)
	m.renderDetailFocus(b, item, summary, titleLookup, episodeTitleIndex, episodes, currentStage, styles, bg)

	// Attention section (always show for failed)
	m.writeSection(b, "Attention", styles, bg)

	// Review reason
	if item.NeedsReview {
		reason := strings.TrimSpace(item.ReviewReason)
		if reason == "" {
			reason = "Needs operator review"
		}
		renderDetailField(b, bg, "Review", styles.WarningText, reason, styles.Text)
	}

	// Error message
	if msg := strings.TrimSpace(item.ErrorMessage); msg != "" {
		renderDetailField(b, bg, "Error", styles.DangerText, msg, styles.Text)
	}

	// Detailed error from Drapto
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
		m.renderMovieScope(b, item, summary, styles, bg)
	} else {
		m.renderEpisodeList(b, item, styles, bg, titleLookup, episodeTitleIndex, currentStage, totals)
	}

	// Validation details (show all steps if there are failures)
	m.renderValidationDetails(b, item, styles, bg)
}

func (m *Model) renderDetailFocus(b *strings.Builder, item spindle.QueueItem, summary spindle.RipSpecSummary, titleLookup map[int]*spindle.RipSpecTitleInfo, episodeTitleIndex map[string]int, episodes []spindle.EpisodeStatus, currentStage string, styles Styles, bg BgStyle) {
	mediaType := detectMediaType(item.Metadata)
	m.writeSection(b, "Focus", styles, bg)
	if len(episodes) > 0 && mediaType != "movie" {
		if isHealthyCompletedTV(item, episodes, spindle.EpisodeTotals{Planned: len(episodes), Final: countFinalEpisodes(episodes)}) {
			renderDetailField(b, bg, "State", styles.MutedText, "Batch complete", styles.Text.Bold(true))
			renderDetailField(b, bg, "Batch", styles.MutedText, fmt.Sprintf("%d planned · %d matched · %d final", len(episodes), matchedEpisodeCount(item, episodes), countFinalEpisodes(episodes)), styles.Text)
			return
		}
		idx, inferred, reason := m.activeEpisodeDescriptor(item, episodes)
		if idx < 0 || idx >= len(episodes) {
			b.WriteString(bg.Render("No episode context available", styles.MutedText))
			b.WriteString("\n")
			return
		}
		ep := episodes[idx]
		stage := m.episodeStage(ep, currentStage, true)
		m.renderEpisodeFocusLine(b, ep, stage, styles, bg)
		b.WriteString("\n")
		label := "Episode: "
		if inferred {
			label = "Likely:  "
		}
		value := formatEpisodeLabel(ep)
		if inferred {
			if focusReason := formatFocusReason(reason); focusReason != "" {
				value += " (" + focusReason + ")"
			}
		}
		renderDetailField(b, bg, strings.TrimSpace(strings.TrimSuffix(label, ":")), styles.MutedText, value, styles.Text)
		if track := m.describeEpisodeTrackInfo(&ep, titleLookup, episodeTitleIndex); track != "" {
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
	m.renderMovieFocus(b, item, summary, currentStage, styles, bg)
}

func countFinalEpisodes(episodes []spindle.EpisodeStatus) int {
	count := 0
	for _, ep := range episodes {
		stage := normalizeEpisodeStage(ep.Stage)
		if stage == "final" || stage == "completed" {
			count++
		}
	}
	return count
}

func formatFocusReason(reason string) string {
	switch reason {
	case "active", "":
		return ""
	case "input match":
		return "matched input"
	case "stage match":
		return "same stage"
	case "ready":
		return "next ready"
	case "next remaining":
		return "next remaining"
	case "last":
		return "last item"
	default:
		return reason
	}
}

func (m *Model) renderMovieFocus(b *strings.Builder, item spindle.QueueItem, summary spindle.RipSpecSummary, currentStage string, styles Styles, bg BgStyle) {
	if cut := m.movieFocusLine(summary, currentStage, styles, bg); cut != "" {
		b.WriteString(cut)
		b.WriteString("\n")
	}
	if len(summary.Titles) > 0 {
		main := summary.Titles[0]
		if main.ID > 0 {
			value := fmt.Sprintf("Title %02d", main.ID)
			if main.Duration > 0 {
				value += " (" + formatRuntime(main.Duration) + ")"
			}
			renderDetailField(b, bg, "Source", styles.MutedText, value, styles.Text)
		}
	}
	if files := m.describeItemFileStates(item); files != "" {
		renderDetailField(b, bg, "Files", styles.MutedText, files, styles.Text)
	}
	if msg := strings.TrimSpace(item.Progress.Message); msg != "" {
		renderDetailField(b, bg, "State", styles.MutedText, msg, styles.Text)
	}
}

func (m *Model) renderMovieScope(b *strings.Builder, item spindle.QueueItem, summary spindle.RipSpecSummary, styles Styles, bg BgStyle) {
	if detectMediaType(item.Metadata) != "movie" {
		return
	}

	m.writeSection(b, "Scope", styles, bg)
	if len(summary.Titles) > 0 {
		main := summary.Titles[0]
		name := strings.TrimSpace(main.Name)
		if name == "" && main.ID > 0 {
			name = fmt.Sprintf("Title %02d", main.ID)
		}
		if name != "" {
			value := name
			if main.Duration > 0 {
				value += " (" + formatRuntime(main.Duration) + ")"
			}
			renderDetailField(b, bg, "Source", styles.MutedText, value, styles.Text)
		}
	}
	if files := m.describeItemFileStates(item); files != "" {
		renderDetailField(b, bg, "Files", styles.MutedText, files, styles.Text)
	}
	stage := normalizeEpisodeStage(item.Stage)
	if stage != "final" && stage != "completed" {
		m.renderSubtitleInfo(b, item, styles, bg)
		m.renderValidationSummary(b, item, styles, bg)
	}
}
