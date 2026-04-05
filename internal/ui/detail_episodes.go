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

	collapsed := m.isEpisodesCollapsed(item, episodes, totals)

	// Section header with toggle hint
	toggleHint := "[t]"
	m.writeSection(b, fmt.Sprintf("Episodes %s", toggleHint), styles, bg)
	m.renderEpisodeSummary(b, item, episodes, totals, styles, bg)

	// Episode sync warning (show when episodes don't have resolved keys)
	if matched := matchedEpisodeCount(item, episodes); matched > 0 && matched < len(episodes) {
		b.WriteString(bg.Render("⚠ Episode numbers not confirmed", styles.WarningText))
		b.WriteString("\n")
	}

	activeIdx := m.activeEpisodeIndex(item, episodes)

	if collapsed {
		b.WriteString(bg.Render("Press t to expand", styles.FaintText))
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
// Defaults to auto-expanding small sets and high-signal states unless explicitly overridden.
func (m *Model) isEpisodesCollapsed(item spindle.QueueItem, episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals) bool {
	collapsed, ok := m.detailState.episodeCollapsed[item.ID]
	if ok {
		return collapsed
	}
	return !shouldAutoExpandEpisodes(item, episodes, totals)
}

func shouldAutoExpandEpisodes(item spindle.QueueItem, episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals) bool {
	if len(episodes) <= 8 {
		return true
	}
	if len(spindle.FilterFailed(episodes)) > 0 {
		return true
	}
	if matched := matchedEpisodeCount(item, episodes); matched > 0 && matched < len(episodes) {
		return true
	}
	stage := normalizeEpisodeStage(item.Stage)
	if (stage == "final" || stage == "completed") && totals.Planned > 0 && totals.Final < totals.Planned {
		return true
	}
	return false
}

func matchedEpisodeCount(item spindle.QueueItem, episodes []spindle.EpisodeStatus) int {
	if item.EpisodeIdentifiedCount > 0 {
		return min(item.EpisodeIdentifiedCount, len(episodes))
	}
	count := 0
	for _, ep := range episodes {
		if isEpisodeMapped(ep) {
			count++
		}
	}
	return count
}

func (m *Model) renderEpisodeSummary(b *strings.Builder, item spindle.QueueItem, episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals, styles Styles, bg BgStyle) {
	parts := []string{fmt.Sprintf("%d planned", totals.Planned)}
	if matched := matchedEpisodeCount(item, episodes); matched > 0 {
		parts = append(parts, fmt.Sprintf("%d matched", matched))
	}
	if totals.Ripped > 0 {
		parts = append(parts, fmt.Sprintf("%d ripped", totals.Ripped))
	}
	if totals.Encoded > 0 {
		parts = append(parts, fmt.Sprintf("%d encoded", totals.Encoded))
	}
	if totals.Subtitled > 0 {
		parts = append(parts, fmt.Sprintf("%d subtitled", totals.Subtitled))
	}
	if totals.Final > 0 {
		parts = append(parts, fmt.Sprintf("%d final", totals.Final))
	}
	if failed := len(spindle.FilterFailed(episodes)); failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	b.WriteString(bg.Render(strings.Join(parts, " · "), styles.MutedText))
	b.WriteString("\n")
}

// renderEpisodeRowEnhanced renders a single episode with stage chip and extras.
func (m *Model) renderEpisodeRowEnhanced(b *strings.Builder, ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int, active bool, stageName string, styles Styles, bg BgStyle) {
	marker := bg.Render("·", styles.FaintText)
	if ep.IsFailed() {
		marker = bg.Render("✗", styles.DangerText)
	} else if active {
		marker = bg.Render(">", styles.AccentText.Bold(true))
	}

	label := formatEpisodeLabel(ep)
	stageChip := m.episodeStageChip(stageName, ep.IsFailed(), styles, bg)
	title, extras, _ := m.describeEpisodeWithExtras(ep, titles, keyLookup)
	titleStyle := styles.Text
	if active {
		titleStyle = styles.Text.Bold(true)
	}

	b.WriteString(marker)
	b.WriteString(bg.Space())
	b.WriteString(bg.Render(label, styles.MutedText))
	b.WriteString(bg.Space())
	b.WriteString(stageChip)
	b.WriteString(bg.Space())
	b.WriteString(bg.Render(title, titleStyle))
	b.WriteString("\n")

	if meta := compactEpisodeMeta(m.describeEpisodeTrackInfo(&ep, titles, keyLookup), describeEpisodeMapping(ep), extras); meta != "" {
		b.WriteString(bg.Spaces(4))
		b.WriteString(bg.Render(meta, styles.FaintText))
		b.WriteString("\n")
	}

	if status := compactEpisodeStatus(m.describeEpisodeFileStates(&ep), describeEpisodeIssue(ep, stageName)); status != "" {
		b.WriteString(bg.Spaces(4))
		b.WriteString(bg.Render(status, styles.MutedText))
		b.WriteString("\n")
	}

	if ep.IsFailed() && ep.ErrorMessage != "" {
		errMsg := ep.ErrorMessage
		if len(errMsg) > 80 {
			errMsg = errMsg[:77] + "..."
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

	if runtime := formatRuntime(ep.RuntimeSeconds); runtime != "" {
		extras = append(extras, runtime)
	}

	lang := strings.TrimSpace(ep.GeneratedSubtitleLanguage)
	if lang == "" {
		lang = strings.TrimSpace(ep.SubtitleLanguage)
	}
	if lang != "" {
		extras = append(extras, strings.ToUpper(lang))
	}

	source := strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleSource))
	if source == "whisperx" {
		extras = append(extras, "AI")
	}

	info := m.lookupRipTitleInfo(&ep, titles, keyLookup)
	return title, extras, info
}

// activeEpisodeIndex finds the currently active episode index.
func (m *Model) activeEpisodeIndex(item spindle.QueueItem, episodes []spindle.EpisodeStatus) int {
	idx, _, _ := m.activeEpisodeDescriptor(item, episodes)
	return idx
}

// activeEpisodeDescriptor describes the best current episode candidate and whether
// it was inferred rather than reported explicitly.
func (m *Model) activeEpisodeDescriptor(item spindle.QueueItem, episodes []spindle.EpisodeStatus) (idx int, inferred bool, reason string) {
	if len(episodes) == 0 {
		return -1, false, ""
	}

	for i, ep := range episodes {
		if ep.Active {
			return i, false, "active"
		}
	}

	stage := itemCurrentStage(item)
	if (stage == "encoding" || stage == "subtitling") && item.Encoding != nil && item.Encoding.InputFile != "" {
		input := item.Encoding.InputFile
		for i, ep := range episodes {
			if checkMatch(input, ep.RippedPath) || checkMatch(input, ep.OutputBasename) {
				return i, true, "input match"
			}
		}
	}

	for i, ep := range episodes {
		if normalizeEpisodeStage(ep.Stage) == stage {
			return i, true, "stage match"
		}
	}

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
				return i, true, "ready"
			}
		}
	}

	for i, ep := range episodes {
		epStage := normalizeEpisodeStage(ep.Stage)
		if epStage != "final" && epStage != "completed" {
			return i, true, "next remaining"
		}
	}

	return len(episodes) - 1, true, "last"
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
		label = "ORG"
	case "subtitled", "subtitling":
		color = m.theme.StatusColors["subtitled"]
		label = "SUB"
	case "encoded", "encoding":
		color = m.theme.StatusColors["encoded"]
		label = "ENC"
	case "audio_analyzed", "audio_analyzing":
		color = m.theme.StatusColors["audio_analyzed"]
		label = "ANLY"
	case "ripped", "ripping":
		color = m.theme.StatusColors["ripped"]
		label = "RIP"
	case "episode_identified", "episode_identifying":
		color = m.theme.StatusColors["episode_identified"]
		label = "MATCH"
	case "identified", "identifying":
		color = m.theme.StatusColors["ripped"]
		label = "ID"
	case "planned":
		color = m.theme.Muted
		label = "PLAN"
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
		parts = append(parts, "RIP")
	}
	if ep.EncodedPath != "" {
		parts = append(parts, "ENC")
	}
	if ep.SubtitledPath != "" {
		parts = append(parts, "SUB")
	}
	if ep.FinalPath != "" {
		parts = append(parts, "FIN")
	}
	return strings.Join(parts, " ")
}

// describeItemFileStates returns file state info for a movie item.
// Uses the "main" episode key from the episodes array (API-provided) or totals.
func (m *Model) describeItemFileStates(item spindle.QueueItem) string {
	// For movies, check the first episode (key "main") or use totals.
	totals := item.EpisodeTotals
	if totals == nil {
		_, derived := item.EpisodeSnapshot()
		totals = &derived
	}
	if totals == nil {
		return ""
	}
	var parts []string
	if totals.Ripped > 0 {
		parts = append(parts, "RIP")
	}
	if totals.Encoded > 0 {
		parts = append(parts, "ENC")
	}
	if totals.Subtitled > 0 {
		parts = append(parts, "SUB")
	}
	if totals.Final > 0 {
		parts = append(parts, "FIN")
	}
	return strings.Join(parts, " ")
}

func describeEpisodeMapping(ep spindle.EpisodeStatus) string {
	switch {
	case ep.MatchScore > 0:
		return fmt.Sprintf("Match %.2f", ep.MatchScore)
	case ep.MatchedEpisode > 0:
		return fmt.Sprintf("Matched E%02d", ep.MatchedEpisode)
	case ep.Episode > 0:
		return "Mapped"
	default:
		return "Unmatched"
	}
}

func describeEpisodeIssue(ep spindle.EpisodeStatus, stageName string) string {
	if ep.IsFailed() {
		return "failed"
	}
	switch strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleDecision)) {
	case "no_match":
		return "subtitle no-match"
	case "error":
		return "subtitle lookup error"
	}
	if isEpisodeMapped(ep) {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(stageName)) {
	case "planned", "identifying", "identified", "ripping", "ripped":
		return ""
	case "episode_identifying":
		return "matching in progress"
	default:
		return "unconfirmed mapping"
	}
}

func compactEpisodeMeta(track string, mapping string, extras []string) string {
	parts := make([]string, 0, 2)
	if track != "" {
		parts = append(parts, track)
	}
	if mapping != "" {
		parts = append(parts, mapping)
	}
	if len(extras) > 0 {
		parts = append(parts, strings.Join(extras, " · "))
	}
	return strings.Join(parts, "  ·  ")
}

func compactEpisodeStatus(files string, issue string) string {
	var parts []string
	if files != "" {
		parts = append(parts, files)
	}
	if issue != "" {
		parts = append(parts, issue)
	}
	return strings.Join(parts, "  ·  ")
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
