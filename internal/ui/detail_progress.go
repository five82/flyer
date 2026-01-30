package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/five82/flyer/internal/spindle"
)

// renderPipelineStatus renders the pipeline progress visualization.
// Matches tview's stage names and logic exactly, including episode counts for TV shows.
func (m *Model) renderPipelineStatus(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	// Stage names use progressive form (-ing)
	stages := []struct {
		id    string
		label string
	}{
		{"planned", "Planning"},
		{"identifying", "Identifying"},
		{"ripped", "Ripping"},
		{"encoded", "Encoding"},
		{"audio_analyzed", "Analyzing"},
		{"subtitled", "Subtitling"},
		{"organizing", "Organizing"},
		{"final", "Completed"},
	}

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

	// For TV episodes, derive counts by inspecting episode stages
	identifyingCount := 0
	audioAnalyzedCount := 0
	subtitledCount := 0
	organizingCount := 0
	if totals.Planned > 0 {
		for _, ep := range episodes {
			epStage := normalizeEpisodeStage(ep.Stage)
			// Identifying is complete if we've moved past it
			switch epStage {
			case "identified", "ripping", "ripped", "episode_identifying", "episode_identified", "encoding", "encoded", "audio_analyzing", "audio_analyzed", "subtitling", "subtitled", "organizing", "final":
				identifyingCount++
			}
			// Audio analysis is complete if we've moved past it
			switch epStage {
			case "audio_analyzed", "subtitling", "subtitled", "organizing", "final":
				audioAnalyzedCount++
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
			b.WriteString(bg.Render(" → ", styles.FaintText))
		}

		// Calculate count for this stage
		count := plannedCount
		if totals.Planned > 0 {
			switch stage.id {
			case "planned":
				count = totals.Planned
			case "identifying":
				count = identifyingCount
			case "ripped":
				count = totals.Ripped
			case "audio_analyzed":
				count = audioAnalyzedCount
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
			count = singleItemPipelineCount(stage.id, item, activePipelineStage, plannedCount)
		}

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

		// Render with or without counts
		if plannedCount > 1 {
			b.WriteString(bg.Render(icon, style))
			b.WriteString(bg.Space())
			b.WriteString(bg.Render(stage.label, labelStyle))
			b.WriteString(bg.Space())
			b.WriteString(bg.Render(fmt.Sprintf("%d/%d", count, plannedCount), styles.MutedText))
		} else {
			b.WriteString(bg.Render(icon+" "+stage.label, style))
		}
	}
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
	return normalizeEpisodeStage(item.Status)
}

// isKnownPipelineStage returns true if the stage maps to a pipeline stage.
func isKnownPipelineStage(stage string) bool {
	switch stage {
	case "planned", "pending", "identifying", "identified", "ripping", "ripped",
		"audio_analyzing", "audio_analyzed",
		"encoding", "encoded", "subtitling", "subtitled",
		"organizing", "final", "completed", "failed":
		return true
	}
	return false
}

// normalizeEpisodeStage normalizes various stage names to a canonical form.
func normalizeEpisodeStage(stage string) string {
	stage = strings.ToLower(strings.TrimSpace(stage))
	switch stage {
	case "episode_identifying":
		return "identifying"
	case "episode_identified":
		return "identified"
	case "completed":
		return "final"
	case "pending":
		return "planned"
	default:
		return stage
	}
}

// pipelineStageForStatus maps a status to a pipeline stage ID.
func pipelineStageForStatus(status string) string {
	switch normalizeEpisodeStage(status) {
	case "planned", "pending":
		return "planned"
	case "identifying", "identified":
		return "identifying"
	case "ripping", "ripped":
		return "ripped"
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
func singleItemPipelineCount(stageID string, item spindle.QueueItem, activeStage string, plannedCount int) int {
	activeNorm := normalizeEpisodeStage(activeStage)

	// If completed/final, all stages are complete
	if activeNorm == "final" {
		return plannedCount
	}

	// Define stage order
	stageOrder := map[string]int{
		"planned":        0,
		"identifying":    1,
		"ripped":         2,
		"encoded":        3,
		"audio_analyzed": 4,
		"subtitled":      5,
		"organizing":     6,
		"final":          7,
	}

	activeIdx, activeOK := stageOrder[activeNorm]
	stageIdx, stageOK := stageOrder[stageID]

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
			substage := strings.ToLower(strings.TrimSpace(enc.Stage))
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
			// Append ETA
			if eta := m.estimateETA(item); eta != "" {
				msg += " • " + eta
			}
		}
		b.WriteString(bg.Render("   ", styles.Text)) // Indent to align with progress bar
		b.WriteString(bg.Render(msg, styles.Text))
		b.WriteString("\n")
	}
}

// estimateETA estimates the remaining time for an operation.
func (m *Model) estimateETA(item spindle.QueueItem) string {
	stage := itemCurrentStage(item)
	// Check encoding ETA first
	if enc := item.Encoding; enc != nil && (stage == "encoding" || stage == "encoded" || stage == "final") {
		if eta := enc.ETADuration(); eta > 0 {
			return "ETA " + formatDuration(eta)
		}
	}
	// Estimate from percent
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
