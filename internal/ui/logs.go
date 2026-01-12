package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/five82/flyer/internal/spindle"
)

// Log source modes
type logSource int

const (
	logSourceDaemon logSource = iota
	logSourceItem
)

// Log refresh constants
const (
	logRefreshInterval = 2 * time.Second
	logFetchTimeout    = 5 * time.Second
	logFetchLimit      = 100
	logBufferLimit     = 2000
)

// logState holds all log-related state.
type logState struct {
	mode        logSource
	rawLines    []string
	follow      bool
	lastRefresh time.Time

	// Cursors for incremental fetching
	streamCursor uint64
	itemCursor   uint64 // Changed to uint64 for /api/logs cursor

	// Filters (apply to both daemon and item logs via /api/logs)
	filterLevel     string
	filterComponent string
	filterLane      string
	filterRequest   string

	// Search
	searchActive   bool
	searchQuery    string
	searchRegex    *regexp.Regexp
	searchInput    textinput.Model
	searchMatches  []int // Line indices that match
	searchMatchIdx int   // Current match index

	// Content caching - skip re-render when unchanged
	contentVersion uint64
	lastRendered   uint64
}

// initLogState initializes the log state.
func (m *Model) initLogState() {
	ti := textinput.New()
	ti.Placeholder = "Search logs..."
	ti.CharLimit = 100

	m.logState = logState{
		mode:   logSourceDaemon,
		follow: true,
	}
	m.logState.searchInput = ti
}

// initLogViewport initializes the log viewport.
func (m *Model) initLogViewport() {
	m.logViewport = viewport.New(m.width-4, m.height-5)
	m.logViewport.Style = lipgloss.NewStyle()
}

// updateLogViewport updates the log viewport with current content.
func (m *Model) updateLogViewport() {
	if m.logViewport.Width == 0 {
		m.initLogViewport()
	}

	// Update dimensions
	// Box height = m.height - 3 (header, cmdbar, status bar below)
	// Box inner = box height - 2 (top and bottom borders) = m.height - 5
	m.logViewport.Width = m.width - 4
	m.logViewport.Height = m.height - 5

	// Ensure viewport has focus background
	m.logViewport.Style = lipgloss.NewStyle().Background(lipgloss.Color(m.theme.FocusBg))

	// Only re-render content if it changed (version mismatch or first render)
	if m.logState.lastRendered == 0 || m.logState.contentVersion != m.logState.lastRendered {
		content := m.renderLogContent()
		m.logViewport.SetContent(content)
		m.logState.lastRendered = m.logState.contentVersion
		if m.logState.lastRendered == 0 {
			m.logState.lastRendered = 1 // Mark as rendered at least once
		}
	}

	// Auto-scroll if following
	if m.logState.follow {
		m.logViewport.GotoBottom()
	}
}

// renderLogs renders the log view.
func (m Model) renderLogs() string {
	// Logs view is always focused when shown, so use FocusBg
	bg := NewBgStyle(m.theme.FocusBg)
	styles := m.theme.Styles()
	contentHeight := m.height - 3 // Account for header + cmdbar + status bar below

	// Title for the box
	title := m.getLogTitle()

	// Viewport content only (no status bar inside)
	content := m.logViewport.View()

	// Logs view is always focused when shown
	box := m.renderBox(title, content, m.width, contentHeight, true)

	// Status bar below the box
	status := m.renderLogStatus(styles, bg)
	return box + "\n" + status
}

// getLogTitle returns the plain text title for the log view.
func (m Model) getLogTitle() string {
	switch m.logState.mode {
	case logSourceItem:
		if item := m.getSelectedItem(); item != nil {
			return fmt.Sprintf("Item #%d Log", item.ID)
		}
		return "Item Log"
	default:
		if m.logFiltersActive() {
			return "Daemon Log (filtered)"
		}
		return "Daemon Log"
	}
}

// renderLogStatus renders the log status bar.
func (m *Model) renderLogStatus(styles Styles, bg BgStyle) string {
	// If we have an active search with matches, show search status instead
	if m.logState.searchRegex != nil && len(m.logState.searchMatches) > 0 {
		matchNum := m.logState.searchMatchIdx + 1
		totalMatches := len(m.logState.searchMatches)
		return bg.Render(fmt.Sprintf("/%s", m.logState.searchQuery), styles.AccentText) +
			bg.Render(" - ", styles.FaintText) +
			bg.Render(fmt.Sprintf("%d/%d", matchNum, totalMatches), styles.WarningText) +
			bg.Render(" - Press ", styles.FaintText) +
			bg.Render("n", styles.AccentText) +
			bg.Render(" for next, ", styles.FaintText) +
			bg.Render("N", styles.AccentText) +
			bg.Render(" for previous, ", styles.FaintText) +
			bg.Render("Esc", styles.AccentText) +
			bg.Render(" to clear", styles.FaintText)
	}

	// If search regex exists but no matches
	if m.logState.searchRegex != nil && len(m.logState.searchMatches) == 0 {
		return bg.Render("Pattern not found: "+m.logState.searchQuery, styles.DangerText)
	}

	// Source label
	var src, apiPath string
	switch m.logState.mode {
	case logSourceItem:
		src = "Item"
		if item := m.getSelectedItem(); item != nil {
			apiPath = fmt.Sprintf("api logs item=%d", item.ID)
		}
	default:
		src = "Daemon"
		apiPath = "api logs"
	}

	// Build status: "Item log 341 lines auto-tail on"
	autoTail := "off"
	if m.logState.follow {
		autoTail = "on"
	}
	status := fmt.Sprintf("%s log %d lines auto-tail %s", src, len(m.logState.rawLines), autoTail)

	var parts []string
	parts = append(parts, bg.Render(status, styles.FaintText))

	// Search input mode
	if m.logState.searchActive {
		parts = append(parts, bg.Render("search: "+m.logState.searchInput.Value(), styles.AccentText))
	}

	// Filters
	if m.logFiltersActive() {
		filterParts := []string{}
		if m.logState.filterLevel != "" {
			filterParts = append(filterParts, "level="+m.logState.filterLevel)
		}
		if m.logState.filterComponent != "" {
			filterParts = append(filterParts, "comp="+m.logState.filterComponent)
		}
		if m.logState.filterLane != "" {
			filterParts = append(filterParts, "lane="+m.logState.filterLane)
		}
		if m.logState.filterRequest != "" {
			filterParts = append(filterParts, "req="+m.logState.filterRequest)
		}
		if len(filterParts) > 0 {
			parts = append(parts, bg.Render("filter: "+strings.Join(filterParts, " "), styles.MutedText))
		}
	}

	// API path at the end
	if apiPath != "" {
		parts = append(parts, bg.Render(apiPath, styles.AccentText))
	}

	// Join with styled bullet separator
	sep := bg.Space() + bg.Render("•", styles.FaintText) + bg.Space()
	content := strings.Join(parts, sep)
	return content
}

// renderLogContent renders the colorized log lines.
func (m *Model) renderLogContent() string {
	// Logs view is always focused when shown, so use FocusBg
	bg := NewBgStyle(m.theme.FocusBg)
	styles := m.theme.Styles()
	width := m.logViewport.Width

	// Empty state for item logs with no item selected
	if m.logState.mode == logSourceItem && m.getSelectedItem() == nil {
		return bg.FillLine(bg.Render("Select an item to view logs", styles.MutedText), width)
	}

	if len(m.logState.rawLines) == 0 {
		return bg.FillLine(bg.Render("No log entries", styles.MutedText), width)
	}

	// Build a set of matching line indices for quick lookup
	matchSet := make(map[int]bool)
	for _, idx := range m.logState.searchMatches {
		matchSet[idx] = true
	}
	activeMatchLine := -1
	if len(m.logState.searchMatches) > 0 && m.logState.searchMatchIdx < len(m.logState.searchMatches) {
		activeMatchLine = m.logState.searchMatches[m.logState.searchMatchIdx]
	}

	var b strings.Builder

	for i, line := range m.logState.rawLines {
		lineNum := i + 1

		// Determine if this line is a search match
		isActiveMatch := i == activeMatchLine
		isPassiveMatch := matchSet[i] && !isActiveMatch

		// Build line content: line number + colorized text
		var lineContent string
		if isActiveMatch {
			// Active match: highlighted background
			highlightBg := NewBgStyle(m.theme.Warning)
			lineContent = highlightBg.Render(fmt.Sprintf("%4d │ ", lineNum), styles.FaintText.Background(lipgloss.Color(m.theme.Warning))) +
				m.colorizeLineForSearch(line, m.theme.Warning)
		} else if isPassiveMatch {
			// Passive match: accent foreground
			lineContent = bg.Render(fmt.Sprintf("%4d │ ", lineNum), styles.AccentText) +
				m.colorizeLineWithHighlight(line, styles, bg)
		} else {
			// Normal line
			lineContent = bg.Render(fmt.Sprintf("%4d │ ", lineNum), styles.FaintText) +
				m.colorizeLineWithBg(line, styles, bg)
		}

		// Pad to fill viewport width
		b.WriteString(bg.FillLine(lineContent, width))
		if i < len(m.logState.rawLines)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// colorizeLineForSearch renders a line with search highlight background.
func (m *Model) colorizeLineForSearch(line string, bgColor string) string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color(bgColor)).
		Foreground(lipgloss.Color(m.theme.Background))
	return style.Render(line)
}

// colorizeLineWithHighlight renders a line with accent foreground for passive matches.
func (m *Model) colorizeLineWithHighlight(line string, styles Styles, bg BgStyle) string {
	return bg.Render(line, styles.AccentText)
}

// colorizeLineWithBg applies Lipgloss styling to a log line using the provided styles.
func (m *Model) colorizeLineWithBg(line string, styles Styles, bg BgStyle) string {
	if strings.TrimSpace(line) == "" {
		return line
	}

	// Detail lines (indented with spaces and starting with key: value or -)
	if content, found := strings.CutPrefix(line, "    "); found {
		if listItem, isList := strings.CutPrefix(content, "- "); isList {
			// List item: "    - File: value"
			return bg.Spaces(8) + bg.Render(listItem, styles.Text)
		}
		// Key-value: "    Key: value"
		return bg.Spaces(4) + bg.Render(content, styles.Text)
	}

	var result strings.Builder
	remaining := line

	// Extract and colorize timestamp (contains space between date and time)
	if matches := timestampRe.FindStringSubmatchIndex(remaining); len(matches) > 0 {
		start, end := matches[2], matches[3]
		ts := remaining[start:end]
		result.WriteString(bg.Render(ts, styles.FaintText))
		remaining = remaining[end:]
	}

	// Extract and colorize log level
	if matches := levelRe.FindStringSubmatchIndex(remaining); len(matches) > 0 {
		start, end := matches[2], matches[3]
		level := remaining[start:end]
		levelStyle := m.getLevelStyle(level, styles)
		result.WriteString(bg.Space())
		result.WriteString(bg.Render(level, levelStyle.Bold(true)))
		remaining = remaining[end:]
	}

	// Skip [component] tag if present (stage is already shown in Item #X (stage))
	if matches := componentRe.FindStringSubmatchIndex(remaining); len(matches) > 0 {
		_, end := matches[0], matches[1]
		remaining = remaining[end:]
	}

	// Extract and colorize item info (contains spaces like "Item #123 (encoding)")
	if matches := itemRe.FindStringSubmatchIndex(remaining); len(matches) > 0 {
		start, end := matches[0], matches[1]
		item := strings.TrimSpace(remaining[start:end])
		result.WriteString(bg.Space())
		result.WriteString(bg.Render(item, styles.AccentText))
		remaining = remaining[end:]
	}

	// Handle separator and message
	if parts := separatorRe.Split(remaining, 2); len(parts) == 2 {
		result.WriteString(bg.Space())
		result.WriteString(bg.Render("–", styles.FaintText))
		result.WriteString(bg.Space())
		result.WriteString(bg.Render(strings.TrimSpace(parts[1]), styles.Text))
	} else {
		result.WriteString(bg.Render(strings.TrimSpace(remaining), styles.Text))
	}

	return result.String()
}

// getLevelStyle returns the style for a log level.
func (m *Model) getLevelStyle(level string, styles Styles) lipgloss.Style {
	switch level {
	case "INFO":
		return styles.SuccessText
	case "WARN":
		return styles.WarningText
	case "ERROR":
		return styles.DangerText
	case "DEBUG":
		return styles.InfoText
	default:
		return styles.Text
	}
}

// logFiltersActive returns true if any log filters are active.
func (m *Model) logFiltersActive() bool {
	return m.logState.filterLevel != "" || m.logState.filterComponent != "" || m.logState.filterLane != "" || m.logState.filterRequest != ""
}

// Regex patterns for log parsing (shared with logtail package)
var (
	timestampRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)
	levelRe     = regexp.MustCompile(`\b(INFO|WARN|ERROR|DEBUG)\b`)
	componentRe = regexp.MustCompile(`\[([^\]]+)\]`)
	itemRe      = regexp.MustCompile(`Item #\d+ \([^)]+\)`)
	separatorRe = regexp.MustCompile(`\s*–\s*`)
)

// handleLogsKey processes keyboard input for logs view.
func (m *Model) handleLogsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search input mode
	if m.logState.searchActive {
		return m.handleLogSearchInput(msg)
	}

	switch {
	case key.Matches(msg, m.keys.ToggleFollow):
		m.logState.follow = !m.logState.follow
		if m.logState.follow {
			m.logViewport.GotoBottom()
		}
		m.updateLogViewport()
		return m, nil

	case key.Matches(msg, m.keys.ToggleLogSource):
		if m.logState.mode == logSourceDaemon {
			m.logState.mode = logSourceItem
		} else {
			m.logState.mode = logSourceDaemon
		}
		m.logState.rawLines = nil
		m.logState.streamCursor = 0
		m.logState.itemCursor = 0
		m.clearLogSearch()
		return m, nil

	case key.Matches(msg, m.keys.Search):
		m.logState.searchActive = true
		m.logState.searchInput.Focus()
		m.logState.searchInput.SetValue("")
		return m, nil

	case key.Matches(msg, m.keys.LogFilters):
		m.openLogFilters()
		return m, nil

	case key.Matches(msg, m.keys.NextMatch):
		m.nextSearchMatch()
		return m, nil

	case key.Matches(msg, m.keys.PrevMatch):
		m.previousSearchMatch()
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		// Clear search if active
		if m.logState.searchRegex != nil {
			m.clearLogSearch()
			m.updateLogViewport()
			return m, nil
		}

	case key.Matches(msg, m.keys.Top):
		m.logViewport.GotoTop()
		m.logState.follow = false
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.logViewport.GotoBottom()
		m.logState.follow = true
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.logViewport.ScrollDown(1)
		m.logState.follow = false
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.logViewport.ScrollUp(1)
		m.logState.follow = false
		return m, nil

	case key.Matches(msg, m.keys.HalfPageDown):
		m.logViewport.HalfPageDown()
		m.logState.follow = false
		return m, nil

	case key.Matches(msg, m.keys.HalfPageUp):
		m.logViewport.HalfPageUp()
		m.logState.follow = false
		return m, nil

	case key.Matches(msg, m.keys.PageDown):
		m.logViewport.PageDown()
		m.logState.follow = false
		return m, nil

	case key.Matches(msg, m.keys.PageUp):
		m.logViewport.PageUp()
		m.logState.follow = false
		return m, nil
	}

	return m, nil
}

// handleLogSearchInput handles keyboard input during log search.
func (m *Model) handleLogSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Confirm):
		// Apply search
		query := m.logState.searchInput.Value()
		if query == "" {
			m.logState.searchActive = false
			m.logState.searchInput.Blur()
			return m, nil
		}

		re, err := regexp.Compile("(?i)" + query)
		if err != nil {
			// Invalid regex - stay in search mode
			return m, nil
		}

		m.logState.searchRegex = re
		m.logState.searchQuery = query
		m.logState.searchActive = false
		m.logState.searchInput.Blur()

		// Find all matches
		m.findSearchMatches()

		// If matches found, scroll to first one
		if len(m.logState.searchMatches) > 0 {
			m.logState.searchMatchIdx = 0
			m.scrollToSearchMatch()
		}

		m.updateLogViewport()
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		// Cancel search input
		m.logState.searchActive = false
		m.logState.searchInput.Blur()
		m.logState.searchInput.SetValue("")
		return m, nil
	}

	// Let the text input handle the key
	var cmd tea.Cmd
	m.logState.searchInput, cmd = m.logState.searchInput.Update(msg)
	return m, cmd
}

// clearLogSearch clears the search state.
func (m *Model) clearLogSearch() {
	m.logState.searchRegex = nil
	m.logState.searchQuery = ""
	m.logState.searchMatches = nil
	m.logState.searchMatchIdx = 0
	m.logState.contentVersion++ // Search highlighting changed
}

// findSearchMatches finds all lines matching the current search regex.
func (m *Model) findSearchMatches() {
	m.logState.searchMatches = nil
	if m.logState.searchRegex == nil {
		return
	}

	for i, line := range m.logState.rawLines {
		if m.logState.searchRegex.MatchString(line) {
			m.logState.searchMatches = append(m.logState.searchMatches, i)
		}
	}
	m.logState.contentVersion++ // Search highlighting changed
}

// nextSearchMatch moves to the next search match.
func (m *Model) nextSearchMatch() {
	if len(m.logState.searchMatches) == 0 {
		return
	}

	m.logState.searchMatchIdx = (m.logState.searchMatchIdx + 1) % len(m.logState.searchMatches)
	m.logState.contentVersion++ // Active match changed
	m.scrollToSearchMatch()
	m.updateLogViewport()
}

// previousSearchMatch moves to the previous search match.
func (m *Model) previousSearchMatch() {
	if len(m.logState.searchMatches) == 0 {
		return
	}

	m.logState.searchMatchIdx = (m.logState.searchMatchIdx - 1 + len(m.logState.searchMatches)) % len(m.logState.searchMatches)
	m.logState.contentVersion++ // Active match changed
	m.scrollToSearchMatch()
	m.updateLogViewport()
}

// scrollToSearchMatch scrolls the viewport to show the current match.
func (m *Model) scrollToSearchMatch() {
	if len(m.logState.searchMatches) == 0 || m.logState.searchMatchIdx >= len(m.logState.searchMatches) {
		return
	}

	targetLine := m.logState.searchMatches[m.logState.searchMatchIdx]
	m.logState.follow = false

	// Calculate scroll position to center the match if possible
	viewportHeight := m.logViewport.Height
	scrollTo := max(targetLine-viewportHeight/2, 0)
	m.logViewport.SetYOffset(scrollTo)
}

// refreshLogs fetches new log entries from the API.
func (m *Model) refreshLogs() tea.Cmd {
	if m.client == nil {
		return nil
	}

	// Don't refresh too frequently
	if time.Since(m.logState.lastRefresh) < logRefreshInterval {
		return nil
	}
	m.logState.lastRefresh = time.Now()

	switch m.logState.mode {
	case logSourceItem:
		return m.fetchItemLogs()
	default:
		return m.fetchDaemonLogs()
	}
}

// fetchDaemonLogs fetches daemon logs from the API.
func (m *Model) fetchDaemonLogs() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), logFetchTimeout)
		defer cancel()

		query := spindle.LogQuery{
			Since:     m.logState.streamCursor,
			Limit:     logFetchLimit,
			Level:     m.logState.filterLevel,
			Component: m.logState.filterComponent,
			Lane:      m.logState.filterLane,
			Request:   m.logState.filterRequest,
		}
		if m.logState.streamCursor == 0 {
			query.Tail = true
		}

		batch, err := m.client.FetchLogs(ctx, query)
		if err != nil {
			return logErrorMsg{err: err}
		}

		return logBatchMsg{
			events: batch.Events,
			next:   batch.Next,
			source: logSourceDaemon,
		}
	}
}

// fetchItemLogs fetches item-specific logs from the streaming API.
// Uses /api/logs with item filter for structured log events.
func (m *Model) fetchItemLogs() tea.Cmd {
	item := m.getSelectedItem()
	if item == nil {
		return nil
	}
	itemID := item.ID

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), logFetchTimeout)
		defer cancel()

		query := spindle.LogQuery{
			Since:     m.logState.itemCursor,
			Limit:     logFetchLimit,
			ItemID:    itemID,
			Level:     m.logState.filterLevel,
			Component: m.logState.filterComponent,
			Lane:      m.logState.filterLane,
			Request:   m.logState.filterRequest,
		}
		if m.logState.itemCursor == 0 {
			query.Tail = true
		}

		batch, err := m.client.FetchLogs(ctx, query)
		if err != nil {
			return logErrorMsg{err: err}
		}

		return logBatchMsg{
			events: batch.Events,
			next:   batch.Next,
			source: logSourceItem,
			itemID: itemID,
		}
	}
}

// Log messages

type logBatchMsg struct {
	events []spindle.LogEvent
	next   uint64
	source logSource
	itemID int64 // For item logs, tracks which item this is for
}

type logErrorMsg struct {
	err error
}

// handleLogBatch processes a batch of log events from the streaming API.
func (m *Model) handleLogBatch(msg logBatchMsg) {
	if msg.source != m.logState.mode {
		return
	}

	// For item logs, verify we're still looking at the same item
	if msg.source == logSourceItem {
		item := m.getSelectedItem()
		if item == nil || item.ID != msg.itemID {
			return
		}
		m.logState.itemCursor = msg.next
	} else {
		m.logState.streamCursor = msg.next
	}

	// Format events to lines
	newLines := formatLogEvents(msg.events)
	if len(newLines) > 0 {
		m.logState.rawLines = append(m.logState.rawLines, newLines...)
		m.logState.rawLines = trimLogBuffer(m.logState.rawLines, logBufferLimit)
		m.logState.contentVersion++ // Mark content changed
		m.updateLogViewport()
	}
}

// formatLogEvents formats log events into display lines.
func formatLogEvents(events []spindle.LogEvent) []string {
	if len(events) == 0 {
		return nil
	}
	lines := make([]string, 0, len(events))
	for _, evt := range events {
		lines = append(lines, formatLogEvent(evt))
	}
	return lines
}

// formatLogEvent formats a single log event.
func formatLogEvent(evt spindle.LogEvent) string {
	ts := evt.Timestamp
	if parsed := evt.ParsedTime(); !parsed.IsZero() {
		ts = parsed.In(time.Local).Format("2006-01-02 15:04:05")
	}
	level := strings.ToUpper(strings.TrimSpace(evt.Level))
	if level == "" {
		level = "INFO"
	}
	parts := []string{ts, level}
	if component := strings.TrimSpace(evt.Component); component != "" {
		parts = append(parts, fmt.Sprintf("[%s]", component))
	}
	subject := composeLogSubject(evt.ItemID, evt.Stage)
	header := strings.Join(parts, " ")
	if subject != "" {
		header += " " + subject
	}
	message := strings.TrimSpace(evt.Message)
	if message != "" {
		header += " – " + message
	}
	if len(evt.Details) == 0 {
		return header
	}
	var builder strings.Builder
	builder.WriteString(header)
	for _, detail := range evt.Details {
		label := strings.TrimSpace(detail.Label)
		value := strings.TrimSpace(detail.Value)
		if label == "" || value == "" {
			continue
		}
		builder.WriteString("\n    - ")
		builder.WriteString(label)
		builder.WriteString(": ")
		builder.WriteString(value)
	}
	return builder.String()
}

// composeLogSubject creates the subject line for a log event.
func composeLogSubject(itemID int64, stage string) string {
	stage = strings.TrimSpace(stage)
	switch {
	case itemID > 0 && stage != "":
		return fmt.Sprintf("Item #%d (%s)", itemID, stage)
	case itemID > 0:
		return fmt.Sprintf("Item #%d", itemID)
	default:
		return stage
	}
}

// trimLogBuffer trims the log buffer to the limit by removing oldest entries.
func trimLogBuffer(lines []string, limit int) []string {
	if overflow := len(lines) - limit; overflow > 0 {
		return append([]string(nil), lines[overflow:]...)
	}
	return lines
}

// --- Log Filters Modal ---

// initLogFilterInputs initializes the text inputs for log filters.
func (m *Model) initLogFilterInputs() {
	// Level input
	levelInput := textinput.New()
	levelInput.Placeholder = "e.g. error, warn, info, debug"
	levelInput.CharLimit = 20
	levelInput.Width = 30

	// Component input
	compInput := textinput.New()
	compInput.Placeholder = "e.g. api, workflow, encoder"
	compInput.CharLimit = 50
	compInput.Width = 30

	// Lane input
	laneInput := textinput.New()
	laneInput.Placeholder = "e.g. ripping, encoding"
	laneInput.CharLimit = 50
	laneInput.Width = 30

	// Request input
	reqInput := textinput.New()
	reqInput.Placeholder = "e.g. abc123"
	reqInput.CharLimit = 50
	reqInput.Width = 30

	m.logFilterInputs[0] = levelInput
	m.logFilterInputs[1] = compInput
	m.logFilterInputs[2] = laneInput
	m.logFilterInputs[3] = reqInput
}

// openLogFilters opens the log filters modal.
func (m *Model) openLogFilters() {
	// Pre-fill with current filter values
	m.logFilterInputs[0].SetValue(m.logState.filterLevel)
	m.logFilterInputs[1].SetValue(m.logState.filterComponent)
	m.logFilterInputs[2].SetValue(m.logState.filterLane)
	m.logFilterInputs[3].SetValue(m.logState.filterRequest)
	m.logFilterFocusIdx = 0
	m.logFilterInputs[0].Focus()
	m.logFilterInputs[1].Blur()
	m.logFilterInputs[2].Blur()
	m.logFilterInputs[3].Blur()
	m.showLogFilters = true
}

// handleLogFiltersKey handles keyboard input for the log filters modal.
func (m Model) handleLogFiltersKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		// Cancel and close modal
		m.showLogFilters = false
		return m, nil

	case key.Matches(msg, m.keys.Confirm):
		// Apply filters and close
		m.applyLogFilters()
		m.showLogFilters = false
		return m, nil

	case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Down):
		// Move to next field
		m.logFilterInputs[m.logFilterFocusIdx].Blur()
		m.logFilterFocusIdx = (m.logFilterFocusIdx + 1) % len(m.logFilterInputs)
		m.logFilterInputs[m.logFilterFocusIdx].Focus()
		return m, nil

	case key.Matches(msg, m.keys.ShiftTab), key.Matches(msg, m.keys.Up):
		// Move to previous field
		m.logFilterInputs[m.logFilterFocusIdx].Blur()
		m.logFilterFocusIdx = (m.logFilterFocusIdx - 1 + len(m.logFilterInputs)) % len(m.logFilterInputs)
		m.logFilterInputs[m.logFilterFocusIdx].Focus()
		return m, nil

	case msg.String() == "ctrl+c":
		// Clear all filters (modal-specific, doesn't quit)
		m.logFilterInputs[0].SetValue("")
		m.logFilterInputs[1].SetValue("")
		m.logFilterInputs[2].SetValue("")
		m.logFilterInputs[3].SetValue("")
		return m, nil
	}

	// Let the focused input handle the key
	var cmd tea.Cmd
	m.logFilterInputs[m.logFilterFocusIdx], cmd = m.logFilterInputs[m.logFilterFocusIdx].Update(msg)
	return m, cmd
}

// applyLogFilters applies the filter values from the modal.
func (m *Model) applyLogFilters() {
	m.logState.filterLevel = strings.TrimSpace(m.logFilterInputs[0].Value())
	m.logState.filterComponent = strings.TrimSpace(m.logFilterInputs[1].Value())
	m.logState.filterLane = strings.TrimSpace(m.logFilterInputs[2].Value())
	m.logState.filterRequest = strings.TrimSpace(m.logFilterInputs[3].Value())

	// Reset log buffer to fetch with new filters
	m.logState.rawLines = nil
	m.logState.streamCursor = 0
	m.logState.itemCursor = 0
	m.clearLogSearch()
}

// renderLogFilters renders the log filters modal.
func (m Model) renderLogFilters() string {
	styles := m.theme.Styles()

	var b strings.Builder

	// Title
	title := styles.Text.Bold(true).Render("Log Filters")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(styles.FaintText.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	// Hint
	b.WriteString(styles.MutedText.Render("Filters apply to both daemon and item logs."))
	b.WriteString("\n")
	b.WriteString(styles.MutedText.Render("Leave blank to disable filter."))
	b.WriteString("\n\n")

	// Level field
	levelLabel := "Level:     "
	if m.logFilterFocusIdx == 0 {
		levelLabel = styles.AccentText.Render(levelLabel)
	} else {
		levelLabel = styles.MutedText.Render(levelLabel)
	}
	b.WriteString(levelLabel)
	b.WriteString(m.logFilterInputs[0].View())
	b.WriteString("\n\n")

	// Component field
	compLabel := "Component: "
	if m.logFilterFocusIdx == 1 {
		compLabel = styles.AccentText.Render(compLabel)
	} else {
		compLabel = styles.MutedText.Render(compLabel)
	}
	b.WriteString(compLabel)
	b.WriteString(m.logFilterInputs[1].View())
	b.WriteString("\n\n")

	// Lane field
	laneLabel := "Lane:      "
	if m.logFilterFocusIdx == 2 {
		laneLabel = styles.AccentText.Render(laneLabel)
	} else {
		laneLabel = styles.MutedText.Render(laneLabel)
	}
	b.WriteString(laneLabel)
	b.WriteString(m.logFilterInputs[2].View())
	b.WriteString("\n\n")

	// Request field
	reqLabel := "Request:   "
	if m.logFilterFocusIdx == 3 {
		reqLabel = styles.AccentText.Render(reqLabel)
	} else {
		reqLabel = styles.MutedText.Render(reqLabel)
	}
	b.WriteString(reqLabel)
	b.WriteString(m.logFilterInputs[3].View())
	b.WriteString("\n\n")

	// Buttons hint
	b.WriteString(styles.FaintText.Render("Enter: Apply  •  Esc: Cancel  •  Ctrl+C: Clear"))

	// Build the modal
	content := b.String()

	// Modal style
	modalWidth := 50
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Accent)).
		Padding(1, 2).
		Width(modalWidth)

	modalContent := modal.Render(content)

	// Center the modal
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modalContent,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color(m.theme.Background)),
	)
}
