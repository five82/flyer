package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/five82/flyer/internal/spindle"
)

// renderEpisodeList renders the episode list for TV content.
// Includes episode sync warning, stage chips, and episode extras.
func (m *Model) renderEpisodeList(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle, titleLookup map[int]*spindle.RipSpecTitleInfo, episodeTitleIndex map[string]int, currentStage string, totals spindle.EpisodeTotals) {
	episodes, _ := item.EpisodeSnapshot()
	if len(episodes) == 0 {
		return
	}

	// Check if collapsed (default is collapsed)
	collapsed := m.isEpisodesCollapsed(item.ID)

	// Section header with toggle hint
	toggleHint := "[t]"
	m.writeSection(b, fmt.Sprintf("Episodes %s", toggleHint), styles, bg)

	// Episode sync warning
	if !item.EpisodesSynced {
		b.WriteString(bg.Render("⚠ Episode numbers not confirmed", styles.WarningText))
		b.WriteString("\n")
	}

	activeIdx := m.activeEpisodeIndex(item, episodes)

	if collapsed {
		b.WriteString(bg.Render(fmt.Sprintf("%d episodes", len(episodes)), styles.MutedText))
		b.WriteString("\n")
		b.WriteString(bg.Render("(press t to expand)", styles.FaintText))
		b.WriteString("\n")
		return
	}

	// Episode list with enhanced rendering
	for idx, ep := range episodes {
		isActive := idx == activeIdx
		stage := m.episodeStage(ep, currentStage, isActive)
		m.renderEpisodeRowEnhanced(b, ep, titleLookup, episodeTitleIndex, isActive, stage, styles, bg)
	}
	b.WriteString(bg.Render("Press t to collapse", styles.FaintText))
	b.WriteString("\n")
}

// isEpisodesCollapsed returns whether episodes are collapsed for an item.
// Default is collapsed (true) unless explicitly expanded.
func (m *Model) isEpisodesCollapsed(itemID int64) bool {
	collapsed, ok := m.detailState.episodeCollapsed[itemID]
	if !ok {
		return true // Default to collapsed
	}
	return collapsed
}

// renderEpisodeRowEnhanced renders a single episode with stage chip and extras.
func (m *Model) renderEpisodeRowEnhanced(b *strings.Builder, ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int, active bool, stageName string, styles Styles, bg BgStyle) {
	// Marker
	marker := bg.Render("·", styles.FaintText)
	if ep.IsFailed() {
		marker = bg.Render("✗", styles.DangerText)
	} else if active {
		marker = bg.Render(">", styles.AccentText.Bold(true))
	}

	// Episode label (S01E01)
	label := formatEpisodeLabel(ep)

	// Stage chip
	stageChip := m.episodeStageChip(stageName, ep.IsFailed(), styles, bg)

	// Title
	title, extras, _ := m.describeEpisodeWithExtras(ep, titles, keyLookup)
	titleStyle := styles.Text
	if active {
		titleStyle = styles.Text.Bold(true)
	}

	// Build the row
	b.WriteString(marker)
	b.WriteString(bg.Space())
	b.WriteString(bg.Render(label, styles.MutedText))
	b.WriteString(bg.Space())
	b.WriteString(stageChip)
	b.WriteString(bg.Space())
	b.WriteString(bg.Render(title, titleStyle))

	// Extras (runtime, language, source) - trim to 2 max
	if len(extras) > 2 {
		extras = extras[:2]
	}
	if len(extras) > 0 {
		b.WriteString(bg.Space())
		b.WriteString(bg.Render("("+strings.Join(extras, " · ")+")", styles.FaintText))
	}
	b.WriteString("\n")

	// Show error message if failed
	if ep.IsFailed() && ep.ErrorMessage != "" {
		errMsg := ep.ErrorMessage
		if len(errMsg) > 60 {
			errMsg = errMsg[:57] + "..."
		}
		b.WriteString(bg.Spaces(4))
		b.WriteString(bg.Render(errMsg, styles.DangerText))
		b.WriteString("\n")
	}
}

// describeEpisodeWithExtras returns title and extra info (runtime, language, source).
func (m *Model) describeEpisodeWithExtras(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) (string, []string, *spindle.RipSpecTitleInfo) {
	title := episodeDisplayTitle(ep)
	var extras []string

	// Runtime
	if runtime := formatRuntime(ep.RuntimeSeconds); runtime != "" {
		extras = append(extras, runtime)
	}

	// Subtitle language
	lang := strings.TrimSpace(ep.GeneratedSubtitleLanguage)
	if lang == "" {
		lang = strings.TrimSpace(ep.SubtitleLanguage)
	}
	if lang != "" {
		extras = append(extras, strings.ToUpper(lang))
	}

	// Subtitle source
	source := strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleSource))
	switch source {
	case "whisperx":
		extras = append(extras, "AI")
		// Check for decision type
		switch strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleDecision)) {
		case "no_match":
			extras = append(extras, "NO-MATCH")
		case "error":
			extras = append(extras, "OS-ERR")
		}
	case "opensubtitles":
		extras = append(extras, "OS")
	}

	info := m.lookupRipTitleInfo(&ep, titles, keyLookup)
	return title, extras, info
}

// activeEpisodeIndex finds the currently active episode index.
func (m *Model) activeEpisodeIndex(item spindle.QueueItem, episodes []spindle.EpisodeStatus) int {
	if len(episodes) == 0 {
		return -1
	}

	// Check for explicitly active episode
	for i, ep := range episodes {
		if ep.Active {
			return i
		}
	}

	stage := itemCurrentStage(item)

	// 1. Precise Match: File path matching
	if stage == "ripping" && item.RippedFile != "" {
		for i, ep := range episodes {
			if checkMatch(item.RippedFile, ep.RippedPath) || checkMatch(item.RippedFile, ep.OutputBasename) {
				return i
			}
		}
	}
	if stage == "encoding" || stage == "subtitling" {
		target := item.EncodedFile
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
		if item.Encoding != nil && item.Encoding.Video != nil && item.Encoding.Video.InputFile != "" {
			input := item.Encoding.Video.InputFile
			for i, ep := range episodes {
				if checkMatch(input, ep.RippedPath) {
					return i
				}
			}
		}
	}

	// 2. Stage Match: Find first episode in the current stage
	for i, ep := range episodes {
		if normalizeEpisodeStage(ep.Stage) == stage {
			return i
		}
	}

	// 3. Pipeline Match: Find first episode ready for the current stage
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

	// 4. Fallback: First non-final episode
	for i, ep := range episodes {
		epStage := normalizeEpisodeStage(ep.Stage)
		if epStage != "final" && epStage != "completed" {
			return i
		}
	}

	return len(episodes) - 1
}

// episodeStage returns the display stage for an episode.
func (m *Model) episodeStage(ep spindle.EpisodeStatus, currentGlobalStage string, isActive bool) string {
	if isActive {
		switch currentGlobalStage {
		case "ripping", "encoding", "identifying", "episode_identifying", "subtitling":
			return currentGlobalStage
		}
	}
	if ep.Stage == "final" {
		return "final"
	}
	return ep.Stage
}

// formatEpisodeLabel formats an episode as S01E01.
func formatEpisodeLabel(ep spindle.EpisodeStatus) string {
	if ep.Season == 0 && ep.Episode == 0 {
		return "S??E??"
	}
	return fmt.Sprintf("S%02dE%02d", ep.Season, ep.Episode)
}

// renderEpisodeFocusLine renders the focused episode line with stage chip.
func (m *Model) renderEpisodeFocusLine(b *strings.Builder, ep spindle.EpisodeStatus, stageName string, styles Styles, bg BgStyle) {
	chip := m.episodeStageChip(stageName, ep.IsFailed(), styles, bg)
	b.WriteString(chip)
	b.WriteString(bg.Space())
	b.WriteString(bg.Render(episodeDisplayTitle(ep), styles.Text.Bold(true)))
}

// episodeStageChip returns a styled chip for an episode stage.
func (m *Model) episodeStageChip(stage string, failed bool, styles Styles, bg BgStyle) string {
	if failed {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.Danger)).
			Padding(0, 1).
			Render("FAIL")
	}

	stage = strings.ToLower(stage)
	color := m.theme.Muted
	label := "WAIT"

	switch stage {
	case "final", "completed":
		color = m.theme.StatusColors["completed"]
		label = "DONE"
	case "organizing":
		color = m.theme.StatusColors["subtitled"]
		label = "ORGZ"
	case "subtitled":
		color = m.theme.StatusColors["subtitled"]
		label = "SUB"
	case "encoded":
		color = m.theme.StatusColors["encoded"]
		label = "ENCD"
	case "audio_analyzed":
		color = m.theme.StatusColors["audio_analyzed"]
		label = "ANLZ"
	case "ripped":
		color = m.theme.StatusColors["ripped"]
		label = "RIPD"
	case "episode_identified":
		color = m.theme.StatusColors["episode_identified"]
		label = "EPID"
	case "identified":
		color = m.theme.StatusColors["ripped"]
		label = "IDNT"
	case "planned":
		color = m.theme.Muted
		label = "PLAN"
	case "encoding", "ripping", "subtitling", "identifying", "episode_identifying":
		color = m.theme.StatusColors[stage]
		label = "WORK"
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Background)).
		Background(lipgloss.Color(color)).
		Padding(0, 1).
		Render(label)
}

// episodeDisplayTitle extracts the display title for an episode.
func episodeDisplayTitle(ep spindle.EpisodeStatus) string {
	if title := strings.TrimSpace(ep.Title); title != "" {
		return title
	}
	if title := strings.TrimSpace(ep.OutputBasename); title != "" {
		return title
	}
	if title := strings.TrimSpace(ep.SourceTitle); title != "" {
		return title
	}
	return "Unlabeled"
}

// describeEpisodeTrackInfo returns track info for an episode.
func (m *Model) describeEpisodeTrackInfo(ep *spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) string {
	info := m.lookupRipTitleInfo(ep, titles, keyLookup)
	var parts []string
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

// lookupRipTitleInfo looks up RipSpec title info for an episode.
func (m *Model) lookupRipTitleInfo(ep *spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) *spindle.RipSpecTitleInfo {
	if ep.SourceTitleID > 0 {
		return titles[ep.SourceTitleID]
	}
	if key, ok := keyLookup[strings.ToLower(ep.Key)]; ok {
		return titles[key]
	}
	return nil
}

// describeEpisodeFileStates returns file state info for an episode.
func (m *Model) describeEpisodeFileStates(ep *spindle.EpisodeStatus) string {
	var parts []string
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

// describeItemFileStates returns file state info for an item (movie).
func (m *Model) describeItemFileStates(item spindle.QueueItem) string {
	var parts []string
	if item.RippedFile != "" {
		parts = append(parts, "Ripped")
	}
	if item.EncodedFile != "" {
		parts = append(parts, "Encoded")
	}
	if item.FinalFile != "" {
		parts = append(parts, "Final")
	}
	return strings.Join(parts, " · ")
}

// movieFocusLine returns the focus line for a movie item.
func (m *Model) movieFocusLine(summary spindle.RipSpecSummary, stage string, styles Styles, bg BgStyle) string {
	if len(summary.Titles) == 0 {
		return ""
	}
	main := summary.Titles[0]
	name := main.Name
	if name == "" {
		name = fmt.Sprintf("Title %02d", main.ID)
	}
	chip := m.episodeStageChip(stage, false, styles, bg)
	return chip + bg.Space() + bg.Render(name, styles.Text)
}
