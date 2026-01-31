package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/five82/flyer/internal/spindle"
)

// compactWidthThreshold is the terminal width below which the UI uses compact mode.
const compactWidthThreshold = 100

// processingStatuses defines which statuses are considered "processing".
// Past-tense statuses like "episode_identified" and "subtitled" are included
// because they represent transitional states where work is still pending.
var processingStatuses = map[string]struct{}{
	"identifying":         {},
	"ripping":             {},
	"audio_analyzing":     {},
	"episode_identifying": {},
	"episode_identified":  {},
	"encoding":            {},
	"subtitling":          {},
	"subtitled":           {},
	"organizing":          {},
}

// renderHeader renders the status bar with all information.
func (m Model) renderHeader() string {
	// Header uses Surface background
	styles := m.theme.Styles().WithBackground(m.theme.Surface)
	bg := NewBgStyle(m.theme.Surface)

	if !m.snapshot.HasStatus || m.snapshot.IsOffline() {
		return m.renderConnectingHeader(styles, bg)
	}

	// Build status parts
	content := m.buildStatusContent(styles, bg)

	// Header bar with background - simple render without complex styling
	return lipgloss.NewStyle().
		Background(lipgloss.Color(m.theme.Surface)).
		Foreground(lipgloss.Color(m.theme.Text)).
		Width(m.width).
		Render(content)
}

// renderConnectingHeader shows the connecting/error state.
func (m Model) renderConnectingHeader(styles Styles, bg BgStyle) string {
	sep := bg.Spaces(2)

	if m.snapshot.LastError != nil {
		last := "soon"
		if !m.lastUpdated.IsZero() {
			last = m.lastUpdated.Format("15:04:05")
		}
		errorMsg := classifyConnectionError(m.snapshot.LastError)

		// Build parts similar to tview: error + retrying + timestamp + log path
		parts := []string{
			bg.Render("flyer", styles.Logo),
			bg.Render("SPINDLE "+errorMsg, styles.DangerText.Bold(true)),
			bg.Render("Retrying...", styles.WarningText.Bold(true)),
			bg.Render(last, styles.MutedText),
		}

		// Add log path hint if config is available
		if m.config != nil {
			logPath := m.config.DaemonLogPath()
			if logPath != "" {
				// Truncate path for display
				displayPath := truncateMiddle(logPath, 50)
				parts = append(parts,
					bg.Render("logs", styles.FaintText)+bg.Space()+
						bg.Render(displayPath, styles.MutedText))
			}
		}

		return styles.Header.Width(m.width).Render(bg.Join(parts, sep))
	}

	return styles.Header.Width(m.width).Render(
		bg.Render("flyer", styles.Logo) + sep +
			bg.Render("Connecting to Spindle...", styles.WarningText.Bold(true)),
	)
}

// countProcessingItems returns the number of items in processing statuses.
func (m Model) countProcessingItems() int {
	stats := m.snapshot.Status.Workflow.QueueStats
	count := 0
	for status := range processingStatuses {
		count += stats[status]
	}
	return count
}

// buildProcessingPart builds the active encoding or processing count display.
// Returns the rendered part and whether an encoding ETA was shown.
func (m Model) buildProcessingPart(compact bool, processing int, styles Styles, bg BgStyle) (string, bool) {
	if encodingItem := m.activeEncodingItem(); encodingItem != nil {
		encodingMini := m.formatEncodingMini(encodingItem, compact, styles, bg)
		showedETA := !compact && encodingItem.Encoding != nil && encodingItem.Encoding.ETADuration() > 0
		return encodingMini, showedETA
	}
	if processing > 0 {
		color := lipgloss.Color(m.theme.StatusColors["encoding"])
		activeStyle := lipgloss.NewStyle().Foreground(color)
		return bg.Render("Active:", styles.MutedText) + bg.Space() +
			bg.Render(fmt.Sprintf("%d", processing), activeStyle), false
	}
	return "", false
}

// buildProblemCountsPart builds the failed/review counts display.
// Returns empty string when both counts are zero.
func (m Model) buildProblemCountsPart(compact bool, failed, review int, styles Styles, bg BgStyle) string {
	if failed == 0 && review == 0 {
		return ""
	}

	sep := bg.Spaces(2)

	failedStyle := styles.MutedText
	if failed > 0 {
		failedStyle = styles.DangerText
	}
	reviewStyle := styles.MutedText
	if review > 0 {
		reviewStyle = styles.WarningText
	}

	failedLabel := "Failed:"
	reviewLabel := "Review:"
	if compact {
		failedLabel = "F:"
		reviewLabel = "R:"
	}

	return bg.Render(failedLabel, styles.MutedText) + bg.Space() + bg.Render(fmt.Sprintf("%d", failed), failedStyle) +
		sep + bg.Render("•", styles.FaintText) + sep +
		bg.Render(reviewLabel, styles.MutedText) + bg.Space() + bg.Render(fmt.Sprintf("%d", review), reviewStyle)
}

// buildErrorParts builds error indicator parts for the header.
func (m Model) buildErrorParts(compact bool, styles Styles, bg BgStyle) []string {
	var parts []string

	if m.snapshot.LastError != nil {
		errText := truncate(fmt.Sprintf("%v", m.snapshot.LastError), maxLen(compact, 80, 40))
		parts = append(parts,
			bg.Render("ERROR", styles.DangerText.Bold(true))+bg.Space()+
				bg.Render(errText, styles.DangerText),
		)
	}

	if m.errorMsg != "" {
		parts = append(parts,
			bg.Render("!", styles.WarningText.Bold(true))+bg.Space()+
				bg.Render(m.errorMsg, styles.WarningText),
		)
	}

	return parts
}

// buildStatusContent builds the status bar content string.
func (m Model) buildStatusContent(styles Styles, bg BgStyle) string {
	compact := m.width < compactWidthThreshold
	processing := m.countProcessingItems()
	failed, review := m.countProblemCounts()

	var parts []string

	// Logo and daemon status
	parts = append(parts, bg.Render("flyer", styles.Logo))
	if m.snapshot.Status.Running {
		parts = append(parts, bg.Render("● ON", styles.SuccessText))
	} else {
		parts = append(parts, bg.Render("● OFF", styles.DangerText))
	}

	// Queue count
	parts = append(parts,
		bg.Render("Queue:", styles.MutedText)+bg.Space()+
			bg.Render(fmt.Sprintf("%d", len(m.snapshot.Queue)), styles.Text),
	)

	// Active encoding or processing count
	processingPart, showedEncodingETA := m.buildProcessingPart(compact, processing, styles, bg)
	if processingPart != "" {
		parts = append(parts, processingPart)
	}

	// Queue ETA (only show if we didn't already show encoding ETA)
	if !showedEncodingETA {
		if eta := m.estimateQueueETA(); eta > 0 {
			parts = append(parts,
				bg.Render("ETA:", styles.MutedText)+bg.Space()+
					bg.Render(formatQueueETA(eta), styles.InfoText),
			)
		}
	}

	// Failed and review counts (only shown when non-zero)
	if problemsPart := m.buildProblemCountsPart(compact, failed, review, styles, bg); problemsPart != "" {
		parts = append(parts, problemsPart)
	}

	// Timestamp
	if timeStr := m.formatTimestamp(); timeStr != "" {
		parts = append(parts, bg.Render(timeStr, styles.MutedText))
	}

	// Health warnings
	if healthWarning := m.formatHealthWarning(compact, styles, bg); healthWarning != "" {
		parts = append(parts, healthWarning)
	}

	// Error indicators
	parts = append(parts, m.buildErrorParts(compact, styles, bg)...)

	return bg.Join(parts, "  ")
}

// countProblemCounts returns the number of failed and review items.
func (m Model) countProblemCounts() (failed, review int) {
	for _, item := range m.snapshot.Queue {
		if strings.EqualFold(item.Status, "failed") {
			failed++
		}
		if item.NeedsReview {
			review++
		}
	}
	return
}

// formatTimestamp formats the last update time with relative indicator.
// Uses compact format (HH:MM) when data is fresh, adds relative time when stale.
func (m Model) formatTimestamp() string {
	if m.lastUpdated.IsZero() {
		return ""
	}

	timeSince := time.Since(m.lastUpdated)

	// Fresh data: just show HH:MM
	if timeSince < time.Minute {
		return m.lastUpdated.Format("15:04")
	}

	// Stale data: show relative time
	if timeSince < time.Hour {
		return fmt.Sprintf("%s (%dm)", m.lastUpdated.Format("15:04"), int(timeSince.Minutes()))
	}
	if timeSince < 24*time.Hour {
		return fmt.Sprintf("%s (%dh)", m.lastUpdated.Format("15:04"), int(timeSince.Hours()))
	}

	// Very stale: full timestamp
	return m.lastUpdated.Format("15:04:05")
}

// formatHealthWarning formats health warnings if any.
func (m Model) formatHealthWarning(compact bool, styles Styles, bg BgStyle) string {
	var unhealthy []string
	for _, sh := range m.snapshot.Status.Workflow.StageHealth {
		if !sh.Ready {
			unhealthy = append(unhealthy, fmt.Sprintf("%s: %s", sh.Name, sh.Detail))
		}
	}
	for _, dep := range m.snapshot.Status.Dependencies {
		if !dep.Available {
			label := dep.Name
			if dep.Detail != "" {
				label += " – " + dep.Detail
			}
			unhealthy = append(unhealthy, label)
		}
	}

	if len(unhealthy) == 0 {
		return ""
	}

	detail := unhealthy[0]
	if len(unhealthy) > 1 {
		detail = fmt.Sprintf("%s +%d more", detail, len(unhealthy)-1)
	}
	detail = truncate(detail, maxLen(compact, 80, 40))

	return bg.Render("HEALTH", styles.DangerText.Bold(true)) + bg.Space() +
		bg.Render(detail, styles.DangerText)
}

// classifyConnectionError returns a short description of the connection error.
func classifyConnectionError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection refused"):
		return "OFFLINE"
	case strings.Contains(msg, "no such host"):
		return "HOST NOT FOUND"
	case strings.Contains(msg, "timeout"):
		return "TIMEOUT"
	default:
		return "ERROR"
	}
}

// renderCommandBar renders the command hints bar (matching tview cmdBar).
func (m Model) renderCommandBar() string {
	// Command bar uses Surface background
	styles := m.theme.Styles().WithBackground(m.theme.Surface)
	bg := NewBgStyle(m.theme.Surface)

	type cmd struct{ key, desc string }
	var commands []cmd

	switch m.currentView {
	case ViewLogs:
		followLabel := "Pause"
		if !m.logState.follow {
			followLabel = "Follow"
		}
		commands = []cmd{
			{"Space", followLabel},
			{"/", "Search"},
			{"n/N", "Next/Prev"},
			{"F", "Filters"},
			{"l", "Daemon"},
			{"i", "Item"},
			{"q", "Queue"},
			{"?", "More"},
		}
	case ViewProblems:
		commands = []cmd{
			{"j/k", "Navigate"},
			{"l", "Daemon"},
			{"i", "Item"},
			{"q", "Queue"},
			{"Tab", "Focus"},
			{"?", "More"},
		}
	default: // ViewQueue
		commands = []cmd{
			{"f", m.filterLabel()}, // Shows current filter state
			{"t", "Episodes"},
			{"P", "Paths"},
			{"j/k", "Navigate"},
			{"l", "Daemon"},
			{"i", "Item"},
			{"p", "Problems"},
			{"Tab", "Focus"},
			{"?", "More"},
		}
	}

	colon := bg.Sep(":")
	sep := bg.Spaces(2)

	segments := make([]string, 0, len(commands))
	for _, c := range commands {
		segments = append(segments,
			bg.Render(c.key, styles.AccentText)+colon+bg.Render(c.desc, styles.MutedText))
	}

	// Show active log search pattern (matching tview)
	if m.currentView == ViewLogs && m.logState.searchQuery != "" {
		pattern := truncate(m.logState.searchQuery, 18)
		segments = append(segments,
			bg.Render("/"+pattern, styles.AccentText))
	}

	// Add theme indicator
	segments = append(segments,
		bg.Render("T", styles.AccentText)+colon+bg.Render(m.theme.Name, styles.FaintText))

	return styles.Header.Width(m.width).Render(strings.Join(segments, sep))
}

// maxLen returns compactLen if compact is true, otherwise normalLen.
func maxLen(compact bool, normalLen, compactLen int) int {
	if compact {
		return compactLen
	}
	return normalLen
}

// truncate truncates a string to max length with ellipsis.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// truncateMiddle truncates a string in the middle, preserving start and end.
func truncateMiddle(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 5 {
		return s[:max]
	}
	// Keep more of the end (file name) than the start
	endLen := (max - 3) * 2 / 3
	startLen := max - 3 - endLen
	return s[:startLen] + "..." + s[len(s)-endLen:]
}

// estimateRemainingFromProgress estimates remaining time based on elapsed time and progress.
// Returns 0 if no reliable estimate can be made.
func estimateRemainingFromProgress(item *spindle.QueueItem) time.Duration {
	if item.Progress.Percent <= 0 || item.Progress.Percent >= 100 {
		return 0
	}
	created := item.ParsedCreatedAt()
	if created.IsZero() {
		return 0
	}
	elapsed := time.Since(created)
	if elapsed <= 0 {
		return 0
	}
	remaining := time.Duration(float64(elapsed) * (100 - item.Progress.Percent) / item.Progress.Percent)
	if remaining <= 0 {
		return 0
	}
	return remaining
}

// estimateQueueETA calculates the total estimated time remaining for the queue.
// Returns 0 if no reliable estimate is available.
func (m Model) estimateQueueETA() time.Duration {
	var total time.Duration
	hasEstimate := false

	for i := range m.snapshot.Queue {
		item := &m.snapshot.Queue[i]
		if strings.EqualFold(item.Status, "completed") || strings.EqualFold(item.Status, "failed") {
			continue
		}

		// Use encoding ETA if available (most accurate)
		if enc := item.Encoding; enc != nil {
			if eta := enc.ETADuration(); eta > 0 {
				total += eta
				hasEstimate = true
				continue
			}
		}

		// Fall back to progress-based estimation
		if remaining := estimateRemainingFromProgress(item); remaining > 0 {
			total += remaining
			hasEstimate = true
		}
	}

	if !hasEstimate {
		return 0
	}
	return total
}

// formatQueueETA formats the queue ETA for display in the header.
func formatQueueETA(d time.Duration) string {
	if d <= 0 {
		return "--"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours >= 24 {
		days := hours / 24
		hours %= 24
		return fmt.Sprintf("~%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("~%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("~%dm", minutes)
}

// activeEncodingItem returns the first item that is actively encoding.
func (m Model) activeEncodingItem() *spindle.QueueItem {
	for i := range m.snapshot.Queue {
		item := &m.snapshot.Queue[i]
		if strings.EqualFold(item.Status, "encoding") && item.Encoding != nil {
			return item
		}
	}
	return nil
}

// formatEncodingMini formats a compact encoding progress display for the header.
// Callers must ensure item is non-nil.
func (m Model) formatEncodingMini(item *spindle.QueueItem, compact bool, styles Styles, bg BgStyle) string {
	if item.Encoding == nil {
		return ""
	}

	enc := item.Encoding
	title := truncate(composeTitle(*item), maxLen(compact, 20, 12))

	// Get percentage
	percent := enc.Percent
	if percent <= 0 && enc.TotalFrames > 0 && enc.CurrentFrame > 0 {
		percent = (float64(enc.CurrentFrame) / float64(enc.TotalFrames)) * 100
	}

	// Build display parts
	encodingColor := lipgloss.Color(m.theme.StatusColors["encoding"])
	iconStyle := lipgloss.NewStyle().Foreground(encodingColor)

	var parts []string
	parts = append(parts, bg.Render("⚙", iconStyle))
	parts = append(parts, bg.Render(title, styles.Text))
	parts = append(parts, bg.Render(fmt.Sprintf("%.0f%%", percent), styles.AccentText))

	// Add ETA if available
	if eta := enc.ETADuration(); eta > 0 {
		if !compact {
			parts = append(parts, bg.Render(formatQueueETA(eta), styles.MutedText))
		}
	}

	return strings.Join(parts, bg.Space())
}
