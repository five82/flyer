package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/five82/flyer/internal/spindle"
)

// stageOrdinal maps every known episode stage to its position in the pipeline.
// Used to determine whether an episode has reached or passed a given stage.
var stageOrdinal = map[string]int{
	"planned":             0,
	"identifying":         1,
	"identified":          2,
	"ripping":             3,
	"ripped":              4,
	"episode_identifying": 5,
	"episode_identified":  6,
	"encoding":            7,
	"encoded":             8,
	"audio_analyzing":     9,
	"audio_analyzed":      10,
	"subtitling":          11,
	"subtitled":           12,
	"organizing":          13,
	"final":               14,
}

// stageAtOrBeyond reports whether epStage is at or beyond threshold in the pipeline.
func stageAtOrBeyond(epStage, threshold string) bool {
	epIdx, ok1 := stageOrdinal[epStage]
	thIdx, ok2 := stageOrdinal[threshold]
	return ok1 && ok2 && epIdx >= thIdx
}

// pipelineStages defines the display stages for the detail view pipeline.
// Each entry's threshold is the earliest stage that counts as "complete" for that row.
var pipelineStages = []struct {
	id          string
	activeLabel string // present tense (shown when incomplete)
	doneLabel   string // past tense (shown when complete)
	threshold   string // episode stage at which this pipeline step is considered done
	tvOnly      bool   // only shown for TV shows (multi-episode items)
}{
	{"planned", "Planning", "Planned", "", false},
	{"identifying", "Identifying", "Identified", "identified", false},
	{"ripped", "Ripping", "Ripped", "ripped", false},
	{"episode_identified", "Ep. Matching", "Ep. Matched", "episode_identified", true},
	{"encoded", "Encoding", "Encoded", "encoded", false},
	{"audio_analyzed", "Analyzing", "Analyzed", "audio_analyzed", false},
	{"subtitled", "Subtitling", "Subtitled", "subtitled", false},
	{"organizing", "Organizing", "Organized", "final", false},
	{"final", "Completed", "Completed", "final", false},
}

// renderPipelineStatus renders the pipeline progress visualization,
// including episode counts for TV shows.
func (m *Model) renderPipelineStatus(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	// Get episode data for counts
	episodes, totals := item.EpisodeSnapshot()

	// Determine active stage from progress or status
	activeStage := itemCurrentStage(item)
	if activeStage == "" {
		activeStage = "planned"
	}
	activePipelineStage := pipelineStageForStatus(activeStage)

	// Determine planned count (at least 1 for movies)
	plannedCount := totals.Planned
	if plannedCount <= 0 {
		plannedCount = 1
	}

	// Pre-compute count width for right-aligned display
	countWidth := len(strconv.Itoa(plannedCount))

	for _, stage := range pipelineStages {
		if stage.tvOnly && totals.Planned <= 0 {
			continue
		}

		// Calculate count for this stage
		count := countEpisodesForPipelineStage(stage.id, stage.threshold, episodes, totals.Planned, item.EpisodeIdentifiedCount, activePipelineStage, plannedCount)

		isComplete := count >= plannedCount
		isCurrent := !isComplete && stage.id == activePipelineStage

		// Determine icon and style
		icon := "○"
		style := styles.MutedText
		labelStyle := styles.MutedText

		switch {
		case isComplete:
			icon = "✓"
			style = styles.SuccessText
			labelStyle = styles.Text.Bold(true)
		case isCurrent:
			icon = "◉"
			style = styles.AccentText
			labelStyle = styles.AccentText.Bold(true)
		case count > 0:
			// Partial progress indicator
			icon = "◐"
			style = styles.WarningText
		}

		// Indent + icon
		b.WriteString(bg.Spaces(2))
		b.WriteString(bg.Render(icon, style))
		b.WriteString(bg.Space())

		// Pick label based on completion state
		label := stage.activeLabel
		if isComplete {
			label = stage.doneLabel
		}
		paddedLabel := fmt.Sprintf("%-12s", label)
		b.WriteString(bg.Render(paddedLabel, labelStyle))

		// Right-aligned count (TV shows only)
		if plannedCount > 1 {
			countStr := fmt.Sprintf("%*d/%d", countWidth, count, plannedCount)
			b.WriteString(bg.Render(countStr, styles.MutedText))
		}

		b.WriteString("\n")
	}
}

func countEpisodesForPipelineStage(stageID, threshold string, episodes []spindle.EpisodeStatus, episodePlanned int, episodeIdentifiedCount int, activePipelineStage string, plannedCount int) int {
	if episodePlanned <= 0 {
		return singleItemPipelineCount(stageID, activePipelineStage, plannedCount)
	}
	if stageID == "planned" {
		return episodePlanned
	}
	// Episode matching completion is data-driven (resolved episode numbers/scores),
	// not asset-stage-driven. During encoding, matched episodes may still report
	// stage="ripped" until encode output exists.
	if stageID == "episode_identified" {
		if episodeIdentifiedCount > 0 {
			return min(episodeIdentifiedCount, episodePlanned)
		}
		count := 0
		for _, ep := range episodes {
			if isEpisodeMapped(ep) {
				count++
			}
		}
		return count
	}
	count := 0
	for _, ep := range episodes {
		if stageAtOrBeyond(normalizeEpisodeStage(ep.Stage), threshold) {
			count++
		}
	}
	return count
}

func isEpisodeMapped(ep spindle.EpisodeStatus) bool {
	if ep.MatchedEpisode > 0 || ep.MatchScore > 0 {
		return true
	}
	return ep.Episode > 0
}

// itemCurrentStage returns the normalized current stage for an item.
// Uses Progress.Stage if it maps to a known pipeline stage, otherwise falls back to Status.
func itemCurrentStage(item spindle.QueueItem) string {
	if stage := normalizeEpisodeStage(item.Progress.Stage); stage != "" {
		// Only use Progress.Stage if it maps to a known pipeline stage
		if isKnownPipelineStage(stage) {
			return stage
		}
	}
	return normalizeEpisodeStage(item.Stage)
}

// isKnownPipelineStage returns true if the stage maps to a pipeline stage.
func isKnownPipelineStage(stage string) bool {
	switch stage {
	case "planned", "identifying", "identified", "ripping", "ripped",
		"episode_identifying", "episode_identified",
		"audio_analyzing", "audio_analyzed",
		"encoding", "encoded", "subtitling", "subtitled",
		"organizing", "final", "completed", "failed",
		// Spindle's raw stage names (normalized by normalizeEpisodeStage)
		"identification", "episode_identification", "audio_analysis":
		return true
	}
	return false
}

// normalizeEpisodeStage normalizes various stage names to a canonical form.
// Maps spindle's raw stage names to flyer's display names.
func normalizeEpisodeStage(stage string) string {
	stage = strings.ToLower(strings.TrimSpace(stage))
	switch stage {
	case "completed":
		return "final"
	case "identification":
		return "identifying"
	case "episode_identification":
		return "episode_identifying"
	case "audio_analysis":
		return "audio_analyzing"
	default:
		return stage
	}
}

// pipelineStageForStatus maps a status to a pipeline stage ID.
func pipelineStageForStatus(status string) string {
	switch normalizeEpisodeStage(status) {
	case "planned":
		return "planned"
	case "identifying", "identified":
		return "identifying"
	case "ripping", "ripped":
		return "ripped"
	case "episode_identifying", "episode_identified":
		return "episode_identified"
	case "audio_analyzing", "audio_analyzed":
		return "audio_analyzed"
	case "encoding", "encoded":
		return "encoded"
	case "subtitling", "subtitled":
		return "subtitled"
	case "organizing":
		return "organizing"
	case "final", "completed":
		return "final"
	default:
		return "planned"
	}
}

// singleItemPipelineCount returns the count for a single-item (movie) pipeline stage.
func singleItemPipelineCount(stageID string, activeStage string, plannedCount int) int {
	activeNorm := normalizeEpisodeStage(activeStage)

	// If completed/final, all stages are complete
	if activeNorm == "final" {
		return plannedCount
	}

	activeIdx, activeOK := stageOrdinal[activeNorm]
	stageIdx, stageOK := stageOrdinal[stageID]

	if !activeOK || !stageOK {
		return 0
	}

	// Only completed stages (before current) have full count
	if stageIdx < activeIdx {
		return plannedCount
	}
	// Current and future stages return 0 so isCurrent logic works
	return 0
}

// renderActiveProgress renders the enhanced progress bar with stage icons and labels.
func (m *Model) renderActiveProgress(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	stage := itemCurrentStage(item)

	percent := clampPercent(item.Progress.Percent)
	var label, icon string
	var color lipgloss.Style

	switch stage {
	case "identifying", "identified":
		label = "IDENTIFYING"
		icon = "🔍"
		color = styles.InfoText
	case "episode_identifying", "episode_identified":
		label = "EP. MATCHING"
		icon = "🔍"
		color = styles.InfoText
	case "ripping", "ripped":
		label = "RIPPING"
		icon = "⏵"
		color = styles.AccentText
	case "audio_analyzing", "audio_analyzed":
		label = "ANALYZING"
		icon = "🔊"
		color = styles.InfoText
	case "encoding", "encoded":
		label = "ENCODING"
		icon = "⚙"
		color = styles.WarningText
		// Check for encoding substage from Drapto
		if enc := item.Encoding; enc != nil {
			substage := strings.ToLower(strings.TrimSpace(enc.Substage))
			switch {
			case strings.Contains(substage, "analysis") || strings.Contains(substage, "crop"):
				label = "ANALYZING"
				icon = "🔍"
			case strings.Contains(substage, "valid"):
				label = "VALIDATING"
				icon = "✓"
			}
			// Use specific encoding percent if valid
			if enc.TotalFrames > 0 && enc.CurrentFrame > 0 {
				p := (float64(enc.CurrentFrame) / float64(enc.TotalFrames)) * 100
				if p > 0 {
					percent = p
				}
			}
		}
	case "subtitling", "subtitled":
		label = "SUBTITLING"
		icon = "💬"
		color = styles.InfoText
	case "organizing":
		label = "ORGANIZING"
		icon = "📁"
		color = styles.SuccessText
	default:
		return // No active progress bar for other stages
	}

	// Progress bar
	bar := m.renderProgressBar(percent, 30, styles, bg)
	b.WriteString(bg.Render(icon+" "+label, color.Bold(true)))
	b.WriteString(bg.Spaces(2))
	b.WriteString(bar)
	b.WriteString(bg.Space())
	b.WriteString(bg.Render(fmt.Sprintf("%3.0f%%", percent), styles.Text))

	// Add ETA for stages that support it
	if eta := m.estimateETA(item); eta != "" {
		b.WriteString(bg.Spaces(2))
		b.WriteString(bg.Render(eta, styles.MutedText))
	}

	// Add byte progress for organizing stage
	if stage == "organizing" && item.Progress.TotalBytes > 0 {
		b.WriteString(bg.Spaces(2))
		b.WriteString(bg.Render(fmt.Sprintf("%s / %s",
			formatBytes(item.Progress.BytesCopied),
			formatBytes(item.Progress.TotalBytes)), styles.MutedText))
	}
	b.WriteString("\n")

	// Progress message line - rendered prominently after the progress bar
	if msg := strings.TrimSpace(item.Progress.Message); msg != "" {
		switch stage {
		case "encoding", "encoded":
			// Append fps to the message line
			if item.Encoding != nil && item.Encoding.FPS > 0 {
				msg += fmt.Sprintf(" • %.0f fps", item.Encoding.FPS)
			}
		case "ripping", "ripped":
			// ETA now shown on progress bar line
		}
		b.WriteString(bg.Render("   ", styles.Text)) // Indent to align with progress bar
		b.WriteString(bg.Render(msg, styles.Text))
		b.WriteString("\n")
	}
}

// estimateETA estimates the remaining time for an operation.
func (m *Model) estimateETA(item spindle.QueueItem) string {
	// Suppress during MakeMKV's brief analyzing sub-phase
	if strings.EqualFold(item.Progress.Stage, "Analyzing") {
		return ""
	}

	stage := itemCurrentStage(item)
	// Check encoding ETA first
	if enc := item.Encoding; enc != nil && (stage == "encoding" || stage == "encoded" || stage == "final") {
		if eta := enc.ETADuration(); eta > 0 {
			return "ETA " + formatDuration(eta)
		}
	}
	// Estimate from percent
	percent := clampPercent(item.Progress.Percent)
	if percent < 5 || percent >= 100 {
		return ""
	}
	// Use stage entry time instead of item creation time
	obs, ok := m.stageFirstSeen[item.ID]
	if !ok || obs.firstSeen.IsZero() {
		return ""
	}
	elapsed := time.Since(obs.firstSeen)
	if elapsed <= 0 {
		return ""
	}
	remaining := time.Duration(float64(elapsed) * (100 - percent) / percent)
	if remaining <= 0 {
		return ""
	}
	return "ETA " + formatDuration(remaining)
}

// renderProgressBar renders a text-based progress bar without percentage text.
// Callers are responsible for adding percentage display as needed.
func (m *Model) renderProgressBar(percent float64, width int, styles Styles, bg BgStyle) string {
	percent = clampPercent(percent)
	filled := min(int(float64(width)*percent/100), width)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bg.Render(bar, styles.AccentText)
}
