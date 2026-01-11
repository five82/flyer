package tea

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// processingStatuses defines which statuses are considered "processing".
var processingStatuses = map[string]struct{}{
	"identifying":         {},
	"ripping":             {},
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

	if !m.snapshot.HasStatus {
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

// buildStatusContent builds the status bar content string.
func (m Model) buildStatusContent(styles Styles, bg BgStyle) string {
	compact := m.width < 100
	sep := bg.Spaces(2)

	// Count processing items
	stats := m.snapshot.Status.Workflow.QueueStats
	processing := 0
	for status := range processingStatuses {
		processing += stats[status]
	}

	// Count failed and review items
	failed, review := m.countProblemCounts()

	// Build parts
	var parts []string

	// Logo
	parts = append(parts, bg.Render("flyer", styles.Logo))

	// Daemon status indicator
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

	// Active count (only if non-zero)
	if processing > 0 {
		color := lipgloss.Color(m.theme.StatusColors["encoding"])
		activeStyle := lipgloss.NewStyle().Foreground(color)
		parts = append(parts,
			bg.Render("Active:", styles.MutedText)+bg.Space()+
				bg.Render(fmt.Sprintf("%d", processing), activeStyle),
		)
	}

	// Failed and Review counts
	failedStyle := styles.MutedText
	if failed > 0 {
		failedStyle = styles.DangerText
	}
	reviewStyle := styles.MutedText
	if review > 0 {
		reviewStyle = styles.WarningText
	}

	if compact {
		parts = append(parts,
			bg.Render("F:", styles.MutedText)+bg.Space()+bg.Render(fmt.Sprintf("%d", failed), failedStyle)+
				sep+bg.Render("•", styles.FaintText)+sep+
				bg.Render("R:", styles.MutedText)+bg.Space()+bg.Render(fmt.Sprintf("%d", review), reviewStyle),
		)
	} else {
		parts = append(parts,
			bg.Render("Failed:", styles.MutedText)+bg.Space()+bg.Render(fmt.Sprintf("%d", failed), failedStyle)+
				sep+bg.Render("•", styles.FaintText)+sep+
				bg.Render("Review:", styles.MutedText)+bg.Space()+bg.Render(fmt.Sprintf("%d", review), reviewStyle),
		)
	}

	// Timestamp with relative time
	timeStr := m.formatTimestamp()
	if timeStr != "" {
		parts = append(parts, bg.Render(timeStr, styles.MutedText))
	}

	// Health warnings
	if healthWarning := m.formatHealthWarning(compact, styles, bg); healthWarning != "" {
		parts = append(parts, healthWarning)
	}

	// Error indicator
	if m.snapshot.LastError != nil {
		maxErr := 80
		if compact {
			maxErr = 40
		}
		errText := truncate(fmt.Sprintf("%v", m.snapshot.LastError), maxErr)
		parts = append(parts,
			bg.Render("ERROR", styles.DangerText.Bold(true))+bg.Space()+
				bg.Render(errText, styles.DangerText),
		)
	}

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
func (m Model) formatTimestamp() string {
	if m.lastUpdated.IsZero() {
		return ""
	}

	timeSince := time.Since(m.lastUpdated)
	timeStr := m.lastUpdated.Format("15:04:05")

	if timeSince < time.Minute {
		timeStr += " (now)"
	} else if timeSince < time.Hour {
		timeStr += fmt.Sprintf(" (%dm ago)", int(timeSince.Minutes()))
	} else if timeSince < 24*time.Hour {
		timeStr += fmt.Sprintf(" (%dh ago)", int(timeSince.Hours()))
	}

	return timeStr
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

	max := 80
	if compact {
		max = 40
	}
	detail = truncate(detail, max)

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
