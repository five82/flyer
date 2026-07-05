package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/five82/flyer/internal/spindle"
)

// compactWidthThreshold is the terminal width below which the UI uses compact mode.
const compactWidthThreshold = 100

// isProcessingItem reports whether an item has live scheduler work.
func isProcessingItem(item spindle.QueueItem) bool {
	return len(item.RunningTasks()) > 0
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

		// Build parts: error + retrying + timestamp + log path
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

// countProcessingItems returns the number of items with running tasks.
func (m Model) countProcessingItems() int {
	count := 0
	for _, item := range m.snapshot.Queue {
		if isProcessingItem(item) {
			count++
		}
	}
	return count
}

// buildResourceStrip renders the scheduler's resource occupancy: which
// task of which item holds each resource, with live percent, and the
// drive-free indicator when the drive is idle. Ordering follows the
// daemon's pipeline template.
func (m Model) buildResourceStrip(compact bool, styles Styles, bg BgStyle) string {
	sched := m.snapshot.Status.Scheduler
	if sched == nil || len(sched.Resources) == 0 {
		// Old daemon or no scheduler info: fall back to a bare active count.
		if n := m.countProcessingItems(); n > 0 {
			return bg.Render("Active:", styles.MutedText) + bg.Space() +
				bg.Render(fmt.Sprintf("%d", n), styles.AccentText)
		}
		return ""
	}

	taskPercent := func(itemID int64, taskType string) (float64, bool) {
		for i := range m.snapshot.Queue {
			if m.snapshot.Queue[i].ID != itemID {
				continue
			}
			for _, t := range m.snapshot.Queue[i].Tasks {
				if t.Type == taskType && t.IsRunning() {
					return t.Progress.Percent, true
				}
			}
		}
		return 0, false
	}

	var parts []string
	for _, name := range resourceOrder(m.snapshot.Status.Pipeline, sched.Resources) {
		res := sched.Resources[name]
		label := resourceLabel(name)

		if res.Used == 0 {
			// Idle resources stay quiet, except the drive: "insert the next
			// disc" is the single most useful signal this header carries.
			if name == "drive" {
				state, style := "FREE", styles.SuccessText
				if disc := m.snapshot.Status.Disc; disc != nil && disc.Paused {
					state, style = "PAUSED", styles.WarningText
				}
				parts = append(parts, bg.Render(label+":", styles.MutedText)+bg.Space()+
					bg.Render(state, style.Bold(true)))
			}
			continue
		}

		for _, h := range res.Holders {
			info := stageDisplay(h.Task)
			seg := bg.Render(label+":", styles.MutedText) + bg.Space() +
				bg.Render(fmt.Sprintf("#%d", h.ItemID), styles.Text) + bg.Space() +
				bg.Render(strings.ToLower(info.label), roleStyle(info.role, styles))
			if pct, ok := taskPercent(h.ItemID, h.Task); ok && pct > 0 && !compact {
				seg += bg.Space() + bg.Render(fmt.Sprintf("%.0f%%", pct), styles.AccentText)
			}
			parts = append(parts, seg)
		}
	}

	if len(parts) == 0 {
		return ""
	}
	sep := bg.Space() + bg.Render("|", styles.FaintText) + bg.Space()
	return strings.Join(parts, sep)
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

	if workflowErr := strings.TrimSpace(m.snapshot.Status.Workflow.LastError); workflowErr != "" {
		errText := truncate(workflowErr, maxLen(compact, 80, 40))
		parts = append(parts,
			bg.Render("WORKFLOW", styles.DangerText.Bold(true))+bg.Space()+
				bg.Render(errText, styles.DangerText),
		)
	}

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

	// Resource occupancy strip (drive/gpu/encode tiers with holders).
	if strip := m.buildResourceStrip(compact, styles, bg); strip != "" {
		parts = append(parts, strip)
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
		if strings.EqualFold(item.Stage, "failed") {
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

// renderCommandBar renders the command hints bar.
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

	// Show active log search pattern
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
