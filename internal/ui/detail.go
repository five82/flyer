package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

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
	m.detailViewport = viewport.New(m.width-4, m.height-6)
	m.detailViewport.Style = lipgloss.NewStyle().
		Background(lipgloss.Color(m.theme.SurfaceAlt))
}

// updateDetailViewport updates the detail viewport content.
func (m *Model) updateDetailViewport() {
	if m.detailViewport.Width == 0 {
		m.initDetailViewport()
	}

	// Update viewport dimensions
	m.detailViewport.Width = m.width - 4
	m.detailViewport.Height = m.height - 6

	// Fullscreen detail is always focused, so use FocusBg
	bgColor := m.theme.FocusBg

	// Ensure viewport has focus background
	m.detailViewport.Style = lipgloss.NewStyle().Background(lipgloss.Color(bgColor))

	// Get selected item
	item := m.getSelectedItem()
	if item == nil {
		m.detailViewport.SetContent(m.theme.Styles().MutedText.Render("Select an item to view details"))
		return
	}

	// Render detail content
	content := m.renderDetailContent(*item, m.detailViewport.Width, bgColor)
	m.detailViewport.SetContent(content)
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
	b.WriteString("\n")

	// Pipeline status
	m.renderPipelineStatus(&b, item, styles, bg)
	b.WriteString("\n")

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

	// Title
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
	colorHex := m.theme.StatusColors[strings.ToLower(item.Status)]
	if colorHex == "" {
		colorHex = m.theme.Muted
	}
	statusColor := lipgloss.Color(colorHex)
	statusChip := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Background)).
		Background(statusColor).
		Padding(0, 1).
		Render(strings.ToUpper(titleCase(item.Status)))
	chips = append(chips, statusChip)

	// Lane chip
	if lane := item.ProcessingLane; lane != "" {
		laneChip := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.Accent)).
			Padding(0, 1).
			Render(strings.ToUpper(lane))
		chips = append(chips, laneChip)
	}

	// Drapto preset badge (GRAIN, etc.)
	if preset := item.DraptoPresetLabel(); preset != "" {
		presetChip := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.StatusColors["pending"])).
			Padding(0, 1).
			Render(strings.ToUpper(preset))
		chips = append(chips, presetChip)
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

	// AI/AI2 badge (WhisperX fallback)
	if item.SubtitleGeneration != nil && item.SubtitleGeneration.FallbackUsed {
		label := "AI"
		if item.SubtitleGeneration.WhisperX > 1 {
			label = fmt.Sprintf("AI%d", item.SubtitleGeneration.WhisperX)
		}
		aiChip := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.Warning)).
			Padding(0, 1).
			Render(label)
		chips = append(chips, aiChip)
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
	b.WriteString(bg.Render(strings.ToUpper(title), styles.MutedText))
	b.WriteString("\n")
	b.WriteString(bg.Render(strings.Repeat("─", 38), styles.FaintText))
	b.WriteString("\n")
}

// determineDetailContext returns the appropriate context for rendering.
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

// renderPendingDetail renders detail for pending items.
func (m *Model) renderPendingDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	m.writeSection(b, "Status", styles, bg)
	b.WriteString(bg.Render("Waiting in queue...", styles.MutedText))
	b.WriteString("\n")

	// Metadata section (if available)
	if metaRows := summarizeMetadata(item.Metadata); len(metaRows) > 0 {
		m.writeSection(b, "Metadata", styles, bg)
		m.renderMetadata(b, metaRows, styles, bg)
	}

	// Source path
	if item.SourcePath != "" {
		m.writeSection(b, "Source", styles, bg)
		b.WriteString(bg.Render(item.SourcePath, styles.Text))
		b.WriteString("\n")
	}
}

// renderActiveDetail renders detail for active/processing items.
func (m *Model) renderActiveDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	// Pre-calculate data
	summary, _ := item.ParseRipSpec()
	titleLookup := make(map[int]*spindle.RipSpecTitleInfo)
	episodeTitleIndex := make(map[string]int)
	for i := range summary.Titles {
		t := summary.Titles[i]
		titleLookup[t.ID] = &t
	}
	for _, ep := range summary.Episodes {
		if ep.TitleID <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(ep.Key))
		if key != "" {
			episodeTitleIndex[key] = ep.TitleID
		}
	}
	episodes, totals := item.EpisodeSnapshot()
	mediaType := detectMediaType(item.Metadata)

	// Enhanced active progress bar with stage icons and labels
	m.renderActiveProgress(b, item, styles, bg)

	// Specs section
	m.writeSection(b, "Specs", styles, bg)
	m.renderVideoSpecs(b, item, styles, bg)
	m.renderAudioInfo(b, item, styles, bg)
	m.renderEncodingConfig(b, item, styles, bg)
	m.renderCropInfo(b, item, styles, bg)

	// Current stage
	currentStage := itemCurrentStage(item)

	// Current episode (TV shows) or current selection (movies)
	if len(episodes) > 0 && mediaType != "movie" {
		activeIdx := m.activeEpisodeIndex(item, episodes)
		if activeIdx >= 0 && activeIdx < len(episodes) {
			ep := episodes[activeIdx]
			m.writeSection(b, "Current Episode", styles, bg)
			stage := m.episodeStage(ep, currentStage, true)
			m.renderEpisodeFocusLine(b, ep, titleLookup, episodeTitleIndex, stage, styles, bg)
			b.WriteString("\n")
			if track := m.describeEpisodeTrackInfo(&ep, titleLookup, episodeTitleIndex); track != "" {
				b.WriteString(bg.Render("Track:   ", styles.MutedText))
				b.WriteString(bg.Render(track, styles.Text))
				b.WriteString("\n")
			}
			if files := m.describeEpisodeFileStates(&ep); files != "" {
				b.WriteString(bg.Render("Files:   ", styles.MutedText))
				b.WriteString(bg.Render(files, styles.Text))
				b.WriteString("\n")
			}
		}
	} else {
		// Movie: show current selection
		if cut := m.movieFocusLine(summary, currentStage, styles, bg); cut != "" {
			m.writeSection(b, "Current Selection", styles, bg)
			b.WriteString(cut)
			b.WriteString("\n")
			if files := m.describeItemFileStates(item); files != "" {
				b.WriteString(bg.Render("Files:   ", styles.MutedText))
				b.WriteString(bg.Render(files, styles.Text))
				b.WriteString("\n")
			}
		}
	}

	// Episode list for TV content
	m.renderEpisodeList(b, item, styles, bg, titleLookup, episodeTitleIndex, currentStage, totals)
}

// renderCompletedDetail renders detail for completed items.
// Matches tview's compact results format exactly.
func (m *Model) renderCompletedDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	// Pre-calculate data
	summary, _ := item.ParseRipSpec()
	titleLookup := make(map[int]*spindle.RipSpecTitleInfo)
	episodeTitleIndex := make(map[string]int)
	for i := range summary.Titles {
		t := summary.Titles[i]
		titleLookup[t.ID] = &t
	}
	for _, ep := range summary.Episodes {
		if ep.TitleID <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(ep.Key))
		if key != "" {
			episodeTitleIndex[key] = ep.TitleID
		}
	}
	episodes, totals := item.EpisodeSnapshot()
	mediaType := detectMediaType(item.Metadata)

	m.writeSection(b, "Results", styles, bg)

	// Size: X → Y (Z% reduction)
	m.renderSizeResult(b, item, styles, bg)

	// Video: resolution HDR
	m.renderVideoSpecs(b, item, styles, bg)

	// Audio: format + commentary
	m.renderAudioInfo(b, item, styles, bg)

	// Config: Preset X • CRF Y • Tune Z
	m.renderEncodingConfig(b, item, styles, bg)

	// Encoded: duration @ speed avg
	m.renderEncodeStats(b, item, styles, bg)

	// Validation: ✓ Passed (N/N checks)
	m.renderValidationSummary(b, item, styles, bg)

	// Subtitle summary (TV shows with multiple episodes) - inline in results
	if len(episodes) > 1 && mediaType != "movie" {
		m.renderSubtitleSummary(b, item, styles, bg)
	}

	// Episode list for TV content
	currentStage := normalizeEpisodeStage(item.Status)
	m.renderEpisodeList(b, item, styles, bg, titleLookup, episodeTitleIndex, currentStage, totals)
}

// renderFailedDetail renders detail for failed/review items.
func (m *Model) renderFailedDetail(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	episodes, _ := item.EpisodeSnapshot()
	mediaType := detectMediaType(item.Metadata)

	// Attention section (always show for failed)
	m.writeSection(b, "Attention", styles, bg)

	// Review reason
	if item.NeedsReview {
		reason := strings.TrimSpace(item.ReviewReason)
		if reason == "" {
			reason = "Needs operator review"
		}
		b.WriteString(bg.Render("Review:   ", styles.WarningText))
		b.WriteString(bg.Render(reason, styles.Text))
		b.WriteString("\n")
	}

	// Error message
	if msg := strings.TrimSpace(item.ErrorMessage); msg != "" {
		b.WriteString(bg.Render("Error:    ", styles.DangerText))
		b.WriteString(bg.Render(msg, styles.Text))
		b.WriteString("\n")
	}

	// Detailed error from Drapto
	if item.Encoding != nil && item.Encoding.Error != nil {
		err := item.Encoding.Error
		if title := strings.TrimSpace(err.Title); title != "" && title != strings.TrimSpace(item.ErrorMessage) {
			b.WriteString(bg.Render("Cause:    ", styles.MutedText))
			b.WriteString(bg.Render(title, styles.Text))
			b.WriteString("\n")
		}
		if ctx := strings.TrimSpace(err.Context); ctx != "" {
			b.WriteString(bg.Render("Context:  ", styles.MutedText))
			b.WriteString(bg.Render(ctx, styles.Text))
			b.WriteString("\n")
		}
		if suggestion := strings.TrimSpace(err.Suggestion); suggestion != "" {
			b.WriteString(bg.Render("Suggest:  ", styles.MutedText))
			b.WriteString(bg.Render(suggestion, styles.SuccessText))
			b.WriteString("\n")
		}
	}

	// Last progress section
	m.writeSection(b, "Last Progress", styles, bg)
	stage := itemCurrentStage(item)
	if stage != "" && stage != "failed" {
		// Capitalize first letter
		stageDisplay := stage
		if len(stage) > 0 {
			stageDisplay = strings.ToUpper(stage[:1]) + stage[1:]
		}
		b.WriteString(bg.Render("Stage:    ", styles.MutedText))
		b.WriteString(bg.Render(stageDisplay, styles.Text))
		b.WriteString("\n")
	}

	if item.Progress.Percent > 0 {
		b.WriteString(bg.Render("Progress: ", styles.MutedText))
		b.WriteString(bg.Render(fmt.Sprintf("%.0f%%", item.Progress.Percent), styles.Text))
		b.WriteString("\n")
	}

	// Show current episode if TV show
	if len(episodes) > 0 && mediaType != "movie" {
		activeIdx := m.activeEpisodeIndex(item, episodes)
		if activeIdx >= 0 && activeIdx < len(episodes) {
			ep := episodes[activeIdx]
			label := formatEpisodeLabel(ep)
			title := strings.TrimSpace(ep.Title)
			if title == "" {
				title = strings.TrimSpace(ep.OutputBasename)
			}
			if title != "" {
				b.WriteString(bg.Render("Episode:  ", styles.MutedText))
				b.WriteString(bg.Render(label, styles.Text))
				b.WriteString(bg.Space())
				b.WriteString(bg.Render(title, styles.Text))
				b.WriteString("\n")
			}
		}
	}

	// Validation details (show all steps if there are failures)
	m.renderValidationDetails(b, item, styles, bg)

	// Paths section (always expanded for debugging)
	m.writeSection(b, "Paths", styles, bg)
	if item.SourcePath != "" {
		b.WriteString(bg.Render("Source:   ", styles.MutedText))
		b.WriteString(bg.Render(item.SourcePath, styles.Text))
		b.WriteString("\n")
	}
	if item.ItemLogPath != "" {
		b.WriteString(bg.Render("Log:      ", styles.MutedText))
		b.WriteString(bg.Render(item.ItemLogPath, styles.Text))
		b.WriteString("\n")
	}
}
