package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/five82/flyer/internal/spindle"
)

// renderEpisodeList renders the episode list for TV content.
// Includes episode sync warning, stage chips, and episode extras.
func (m *Model) renderEpisodeList(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle, totals spindle.EpisodeTotals) {
	episodes, _ := item.EpisodeSnapshot()
	if len(episodes) == 0 {
		return
	}

	collapsed := m.isEpisodesCollapsed(item, episodes, totals)

	// Section header with toggle hint
	m.writeSection(b, "Episodes [t]", styles, bg)
	m.renderEpisodeSummary(b, item, episodes, totals, styles, bg)

	// Episode sync warning (show when episodes don't have resolved keys)
	if matched := matchedEpisodeCount(item, episodes); matched > 0 && matched < len(episodes) {
		b.WriteString(bg.Render("⚠ Episode numbers not confirmed", styles.WarningText))
		b.WriteString("\n")
	}

	if collapsed {
		b.WriteString(bg.Render("Press t to expand", styles.FaintText))
		b.WriteString("\n")
		return
	}

	activeIdx, _ := activeEpisodeIndex(item, episodes)

	// Episode list with enhanced rendering
	for idx, ep := range episodes {
		m.renderEpisodeRowEnhanced(b, ep, idx == activeIdx, styles, bg)
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
	if strings.EqualFold(item.Stage, "completed") && totals.Planned > 0 && totals.Final < totals.Planned {
		return true
	}
	return false
}

// isEpisodeMapped reports whether an episode has been resolved to a real
// episode number (vs. still being a placeholder awaiting identification).
func isEpisodeMapped(ep spindle.EpisodeStatus) bool {
	if ep.MatchedEpisode > 0 || ep.MatchScore > 0 {
		return true
	}
	return ep.Episode > 0
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
func (m *Model) renderEpisodeRowEnhanced(b *strings.Builder, ep spindle.EpisodeStatus, active bool, styles Styles, bg BgStyle) {
	marker := bg.Render("·", styles.FaintText)
	if ep.IsFailed() {
		marker = bg.Render("✗", styles.DangerText)
	} else if active {
		marker = bg.Render(">", styles.AccentText.Bold(true))
	}

	label := formatEpisodeLabel(ep)
	stageChip := m.episodeStageChip(ep.Stage, ep.IsFailed(), styles, bg)
	title, extras := describeEpisodeWithExtras(ep)
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

	if meta := compactEpisodeMeta(describeEpisodeTrackInfo(&ep), describeEpisodeMapping(ep), extras); meta != "" {
		b.WriteString(bg.Spaces(4))
		b.WriteString(bg.Render(meta, styles.FaintText))
		b.WriteString("\n")
	}

	if status := compactEpisodeStatus(m.describeEpisodeFileStates(&ep), describeEpisodeIssue(ep)); status != "" {
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
func describeEpisodeWithExtras(ep spindle.EpisodeStatus) (string, []string) {
	title := episodeDisplayTitle(ep)
	var extras []string

	if runtime := formatRuntime(ep.RuntimeSeconds); runtime != "" {
		extras = append(extras, runtime)
	}

	if lang := strings.TrimSpace(ep.SubtitleLanguage); lang != "" {
		extras = append(extras, strings.ToUpper(lang))
	}

	if strings.EqualFold(strings.TrimSpace(ep.SubtitleSource), "whisperx") {
		extras = append(extras, "AI")
	}

	return title, extras
}

// activeEpisodeIndex returns the index of the first episode the server
// reports as actively being worked on: either its Active flag, or a running
// task's activeAssetKey matching the episode's key. When several episodes
// are active at once (e.g. rip/encode overlap on different episodes), the
// first one found wins.
func activeEpisodeIndex(item spindle.QueueItem, episodes []spindle.EpisodeStatus) (int, bool) {
	keys := item.ActiveAssetKeys()
	for i, ep := range episodes {
		if ep.Active || keys[strings.ToLower(ep.Key)] {
			return i, true
		}
	}
	return -1, false
}

// formatEpisodeLabel formats an episode as S01E01.
func formatEpisodeLabel(ep spindle.EpisodeStatus) string {
	if ep.Season == 0 && ep.Episode == 0 {
		return "S??E??"
	}
	return fmt.Sprintf("S%02dE%02d", ep.Season, ep.Episode)
}

// renderEpisodeFocusLine renders the focused episode line with stage chip.
func (m *Model) renderEpisodeFocusLine(b *strings.Builder, ep spindle.EpisodeStatus, styles Styles, bg BgStyle) {
	chip := m.episodeStageChip(ep.Stage, ep.IsFailed(), styles, bg)
	b.WriteString(chip)
	b.WriteString(bg.Space())
	b.WriteString(bg.Render(episodeDisplayTitle(ep), styles.Text.Bold(true)))
}

// episodeStageChip returns a styled chip for an episode's asset stage. This
// is spindle's per-episode vocabulary (planned/ripped/encoded/subtitled/
// final, plus failed) -- a different, legitimate set of names from the
// scheduler's task types in stages.go, keyed directly on the API value.
func (m *Model) episodeStageChip(stage string, failed bool, styles Styles, bg BgStyle) string {
	if failed {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.Background)).
			Background(lipgloss.Color(m.theme.Danger)).
			Padding(0, 1).
			Render("FAIL")
	}

	color := m.theme.Muted
	label := "WAIT"

	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "final":
		color = m.theme.Success
		label = "DONE"
	case "subtitled":
		color = m.theme.Success
		label = "SUB"
	case "encoded":
		color = m.theme.Accent
		label = "ENC"
	case "ripped":
		color = m.theme.Info
		label = "RIP"
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

// describeEpisodeTrackInfo returns source track info for an episode.
func describeEpisodeTrackInfo(ep *spindle.EpisodeStatus) string {
	var parts []string
	if ep.SourceTitleID > 0 {
		parts = append(parts, fmt.Sprintf("Title %02d", ep.SourceTitleID))
	}
	if runtime := formatRuntime(ep.RuntimeSeconds); runtime != "" {
		parts = append(parts, runtime)
	}
	return strings.Join(parts, "  ")
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

// describeItemFileStates returns file state info for a movie item, tallied
// from the item's (usually single) episode asset paths.
func (m *Model) describeItemFileStates(item spindle.QueueItem) string {
	_, totals := item.EpisodeSnapshot()
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

// describeEpisodeIssue reports the episode's problem, if any, straight from
// the API's authoritative per-episode fields -- no client-side guessing.
func describeEpisodeIssue(ep spindle.EpisodeStatus) string {
	if ep.IsFailed() {
		return "failed"
	}
	if ep.NeedsReview {
		if reason := strings.TrimSpace(ep.ReviewReason); reason != "" {
			return reason
		}
		return "needs review"
	}
	return ""
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
