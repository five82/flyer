package tea

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/five82/flyer/internal/spindle"
)

// Problems refresh constants
const (
	problemsRefreshInterval = 2 * time.Second
	problemsFetchTimeout    = 5 * time.Second
	problemsFetchLimit      = 100
	problemsBufferLimit     = 500
)

// problemsState holds state for the problems view.
type problemsState struct {
	logLines    []string
	logCursor   map[string]uint64
	lastItemID  int64
	lastRefresh time.Time
}

// initProblemsViewport initializes the problems viewport.
func (m *Model) initProblemsViewport() {
	m.problemsViewport = viewport.New(m.width-4, m.height-6)
	m.problemsViewport.Style = lipgloss.NewStyle()
	m.problemsState = problemsState{
		logCursor: make(map[string]uint64),
	}
}

// updateProblemsViewport updates the problems viewport with current content.
func (m *Model) updateProblemsViewport() {
	if m.problemsViewport.Width == 0 {
		m.initProblemsViewport()
	}

	// Update dimensions
	m.problemsViewport.Width = m.width - 4
	m.problemsViewport.Height = m.height - 6

	// Ensure viewport has focus background (problems view is always focused when shown)
	m.problemsViewport.Style = lipgloss.NewStyle().Background(lipgloss.Color(m.theme.FocusBg))

	// Render problems content
	content := m.renderProblemsContent()
	m.problemsViewport.SetContent(content)
}

// renderProblems renders the problems view.
func (m Model) renderProblems() string {
	contentHeight := m.height - 2 // Account for header + cmdbar

	// Viewport content
	content := m.problemsViewport.View()

	// Title with item ID if selected
	title := m.getProblemsTitle()

	// Problems view is always focused when shown
	return m.renderTitledBox(title, content, m.width, contentHeight, true)
}

// getProblemsTitle returns the title for the problems view.
func (m Model) getProblemsTitle() string {
	if item := m.getSelectedItem(); item != nil {
		return fmt.Sprintf("Problems (Item #%d)", item.ID)
	}
	return "Problems"
}

// renderProblemsContent renders the full problems content for an item.
func (m *Model) renderProblemsContent() string {
	item := m.getSelectedItem()
	bg := NewBgStyle(m.theme.FocusBg)
	styles := m.theme.Styles().WithBackground(m.theme.FocusBg)
	width := m.problemsViewport.Width

	if item == nil {
		return bg.FillLine(bg.Render("Select an item to view problems", styles.MutedText), width)
	}

	var b strings.Builder

	// Build structured problems
	m.renderStructuredProblems(&b, item, styles, bg)

	// Add log messages section if we have any
	if len(m.problemsState.logLines) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(bg.Render("Log Messages", styles.MutedText.Bold(true)))
		b.WriteString("\n")

		for i, line := range m.problemsState.logLines {
			lineNum := i + 1
			b.WriteString(bg.Render(fmt.Sprintf("%4d │ ", lineNum), styles.FaintText))
			b.WriteString(m.colorizeLineWithBg(line, styles, bg))
			b.WriteString("\n")
		}
	}

	if b.Len() == 0 && len(m.problemsState.logLines) == 0 {
		return bg.FillLine(bg.Render("No warnings or errors for this item", styles.MutedText), width)
	}

	// Fill each line to viewport width
	content := b.String()
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = bg.FillLine(line, width)
	}
	return strings.Join(lines, "\n")
}

// renderStructuredProblems extracts problem info from the item's structured data.
func (m *Model) renderStructuredProblems(b *strings.Builder, item *spindle.QueueItem, styles Styles, bg BgStyle) {
	// Review reason
	if item.NeedsReview && strings.TrimSpace(item.ReviewReason) != "" {
		m.renderProblemSection(b, "Review Reason", styles.WarningText, styles, bg)
		b.WriteString(bg.Spaces(2))
		b.WriteString(bg.Render(item.ReviewReason, styles.Text))
		b.WriteString("\n\n")
	}

	// Error message
	if msg := strings.TrimSpace(item.ErrorMessage); msg != "" {
		m.renderProblemSection(b, "Error", styles.DangerText, styles, bg)
		b.WriteString(bg.Spaces(2))
		b.WriteString(bg.Render(msg, styles.Text))
		b.WriteString("\n\n")
	}

	// Per-episode errors
	failedEpisodes := spindle.FilterFailed(item.Episodes)
	if len(failedEpisodes) > 0 {
		m.renderProblemSection(b, "Failed Episodes", styles.DangerText, styles, bg)
		for _, ep := range failedEpisodes {
			epLabel := ep.Key
			if ep.Title != "" {
				epLabel = fmt.Sprintf("%s - %s", ep.Key, ep.Title)
			}
			b.WriteString(bg.Spaces(2))
			b.WriteString(bg.Render("✗", styles.DangerText))
			b.WriteString(bg.Space())
			b.WriteString(bg.Render(epLabel, styles.Text))
			b.WriteString("\n")
			if msg := strings.TrimSpace(ep.ErrorMessage); msg != "" {
				b.WriteString(bg.Spaces(6))
				b.WriteString(bg.Render(msg, styles.MutedText))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	// Encoding error details
	if item.Encoding != nil && item.Encoding.Error != nil {
		err := item.Encoding.Error
		if strings.TrimSpace(err.Title) != "" || strings.TrimSpace(err.Message) != "" {
			m.renderProblemSection(b, "Encoding Error", styles.DangerText, styles, bg)
			if err.Title != "" {
				b.WriteString(bg.Spaces(2))
				b.WriteString(bg.Render(err.Title, styles.Text))
				b.WriteString("\n")
			}
			if err.Message != "" {
				b.WriteString(bg.Spaces(2))
				b.WriteString(bg.Render(err.Message, styles.MutedText))
				b.WriteString("\n")
			}
			if err.Context != "" {
				b.WriteString(bg.Spaces(2))
				b.WriteString(bg.Render("Context:", styles.FaintText) + bg.Space())
				b.WriteString(bg.Render(err.Context, styles.Text))
				b.WriteString("\n")
			}
			if err.Suggestion != "" {
				b.WriteString(bg.Spaces(2))
				b.WriteString(bg.Render("Suggestion:", styles.FaintText) + bg.Space())
				b.WriteString(bg.Render(err.Suggestion, styles.SuccessText))
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	// Encoding warning
	if item.Encoding != nil && strings.TrimSpace(item.Encoding.Warning) != "" {
		m.renderProblemSection(b, "Warning", styles.WarningText, styles, bg)
		b.WriteString(bg.Spaces(2))
		b.WriteString(bg.Render(item.Encoding.Warning, styles.Text))
		b.WriteString("\n\n")
	}

	// Validation steps
	if item.Encoding != nil && item.Encoding.Validation != nil {
		val := item.Encoding.Validation
		// Only show validation section if it failed or has steps to show
		if !val.Passed || len(val.Steps) > 0 {
			var titleStyle lipgloss.Style
			var passedIcon string
			if val.Passed {
				passedIcon = "✓"
				titleStyle = styles.SuccessText
			} else {
				passedIcon = "✗"
				titleStyle = styles.DangerText
			}

			b.WriteString(bg.Render("Validation", styles.MutedText.Bold(true)))
			b.WriteString(bg.Space())
			b.WriteString(bg.Render(passedIcon, titleStyle))
			b.WriteString("\n")

			for _, step := range val.Steps {
				var icon string
				var iconStyle lipgloss.Style
				if step.Passed {
					icon = "✓"
					iconStyle = styles.SuccessText
				} else {
					icon = "✗"
					iconStyle = styles.DangerText
				}
				b.WriteString(bg.Spaces(2))
				b.WriteString(bg.Render(icon, iconStyle))
				b.WriteString(bg.Space())
				b.WriteString(bg.Render(step.Name, styles.Text))
				b.WriteString("\n")
				if strings.TrimSpace(step.Details) != "" {
					b.WriteString(bg.Spaces(6))
					b.WriteString(bg.Render(step.Details, styles.MutedText))
					b.WriteString("\n")
				}
			}
			b.WriteString("\n")
		}
	}
}

// renderProblemSection renders a section header for problems.
func (m *Model) renderProblemSection(b *strings.Builder, title string, titleStyle lipgloss.Style, _ Styles, bg BgStyle) {
	b.WriteString(bg.Render(title, titleStyle.Bold(true)))
	b.WriteString("\n")
}

// handleProblemsKey processes keyboard input for problems view.
func (m Model) handleProblemsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "g":
		m.problemsViewport.GotoTop()
		return m, nil

	case "G":
		m.problemsViewport.GotoBottom()
		return m, nil

	case "j", "down":
		m.problemsViewport.ScrollDown(1)
		return m, nil

	case "k", "up":
		m.problemsViewport.ScrollUp(1)
		return m, nil

	case "ctrl+d":
		m.problemsViewport.HalfPageDown()
		return m, nil

	case "ctrl+u":
		m.problemsViewport.HalfPageUp()
		return m, nil

	case "pgdown", " ":
		m.problemsViewport.PageDown()
		return m, nil

	case "pgup":
		m.problemsViewport.PageUp()
		return m, nil
	}

	return m, nil
}

// --- Problems Log Fetching ---

// refreshProblemsLogs fetches warn/error logs for the selected item.
func (m *Model) refreshProblemsLogs() tea.Cmd {
	if m.client == nil {
		return nil
	}

	item := m.getSelectedItem()
	if item == nil {
		return nil
	}

	// Clear logs if item changed
	if item.ID != m.problemsState.lastItemID {
		m.problemsState.logLines = nil
		m.problemsState.logCursor = make(map[string]uint64)
		m.problemsState.lastItemID = item.ID
	}

	// Don't refresh too frequently
	if time.Since(m.problemsState.lastRefresh) < problemsRefreshInterval {
		return nil
	}
	m.problemsState.lastRefresh = time.Now()

	itemID := item.ID
	cursor := m.problemsState.logCursor[fmt.Sprintf("%d", itemID)]

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), problemsFetchTimeout)
		defer cancel()

		query := spindle.LogQuery{
			Since:  cursor,
			Limit:  problemsFetchLimit,
			ItemID: itemID,
			Level:  "warn", // Fetch warn and error level logs
		}
		if cursor == 0 {
			query.Tail = true
		}

		batch, err := m.client.FetchLogs(ctx, query)
		if err != nil {
			return problemsLogErrorMsg{err: err}
		}

		return problemsLogBatchMsg{
			events: batch.Events,
			next:   batch.Next,
			itemID: itemID,
		}
	}
}

// Problems log messages

type problemsLogBatchMsg struct {
	events []spindle.LogEvent
	next   uint64
	itemID int64
}

type problemsLogErrorMsg struct {
	err error
}

// handleProblemsLogBatch processes a batch of problems log events.
func (m *Model) handleProblemsLogBatch(msg problemsLogBatchMsg) {
	// Ignore if for a different item
	item := m.getSelectedItem()
	if item == nil || item.ID != msg.itemID {
		return
	}

	// Update cursor for this item
	m.problemsState.logCursor[fmt.Sprintf("%d", msg.itemID)] = msg.next

	// Format events to lines
	newLines := formatLogEvents(msg.events)
	if len(newLines) > 0 {
		m.problemsState.logLines = append(m.problemsState.logLines, newLines...)
		m.problemsState.logLines = trimLogBuffer(m.problemsState.logLines, problemsBufferLimit)
		m.updateProblemsViewport()
	}
}
