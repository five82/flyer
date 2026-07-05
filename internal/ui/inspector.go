package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/five82/flyer/internal/spindle"
)

// inspectorTabLabels are the tab bar entries in tab order.
var inspectorTabLabels = [tabCount]string{"Overview", "Episodes", "Problems", "Logs"}

// getInspectedItem resolves the inspected item by ID from the full queue,
// independent of filters and sorting. Returns nil when the item is gone.
func (m *Model) getInspectedItem() *spindle.QueueItem {
	if m.inspectedID == 0 {
		return nil
	}
	for i := range m.snapshot.Queue {
		if m.snapshot.Queue[i].ID == m.inspectedID {
			return &m.snapshot.Queue[i]
		}
	}
	return nil
}

// openInspector opens the full-screen inspector for the current selection.
func (m Model) openInspector(tab inspectorTab) (tea.Model, tea.Cmd) {
	var item *spindle.QueueItem
	if m.currentView == ViewProblems {
		item = m.getTriageItem()
	} else {
		item = m.getSelectedItem()
	}
	if item == nil {
		return m, nil
	}

	m.returnView = m.currentView
	m.inspecting = true
	m.inspectedID = item.ID
	return m.switchInspectorTab(tab)
}

// closeInspector returns to the view the inspector was opened from.
func (m *Model) closeInspector() {
	m.inspecting = false
	m.currentView = m.returnView
}

// switchInspectorTab activates a tab and kicks off its data fetch if needed.
func (m Model) switchInspectorTab(tab inspectorTab) (tea.Model, tea.Cmd) {
	m.inspectorTab = tab
	item := m.getInspectedItem()

	switch tab {
	case tabLogs:
		if m.logState.mode != logSourceItem {
			m.logState.mode = logSourceItem
			m.logState.rawLines = nil
			m.logState.itemCursor = 0
			m.logState.lastItemID = 0 // Force reset in fetchItemLogs
			m.clearLogSearch()
			m.logState.contentVersion++
		}
		m.updateLogViewport()
		return m, m.refreshLogs(item)
	case tabProblems:
		m.inspectorViewport.GotoTop()
		m.updateInspectorViewport()
		return m, m.refreshProblemsLogs(item)
	default:
		m.inspectorViewport.GotoTop()
		m.updateInspectorViewport()
		return m, nil
	}
}

// handleInspectorKey processes keyboard input while the inspector is open.
func (m Model) handleInspectorKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		// Let an active log search clear first
		if m.inspectorTab == tabLogs && m.logState.searchRegex != nil {
			return m.handleLogsKey(msg)
		}
		m.closeInspector()
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		return m.switchInspectorTab((m.inspectorTab + 1) % tabCount)

	case key.Matches(msg, m.keys.ShiftTab):
		return m.switchInspectorTab((m.inspectorTab + tabCount - 1) % tabCount)

	case key.Matches(msg, m.keys.Tab1):
		return m.switchInspectorTab(tabOverview)

	case key.Matches(msg, m.keys.Tab2):
		return m.switchInspectorTab(tabEpisodes)

	case key.Matches(msg, m.keys.Tab3):
		return m.switchInspectorTab(tabProblems)

	case key.Matches(msg, m.keys.Tab4):
		return m.switchInspectorTab(tabLogs)

	case key.Matches(msg, m.keys.ToggleEpisodes):
		m.toggleInspectedEpisodes()
		return m, nil
	}

	// Logs tab: delegate to the log key handler (follow, search, scroll)
	if m.inspectorTab == tabLogs {
		return m.handleLogsKey(msg)
	}

	// Other tabs scroll the inspector viewport
	switch {
	case key.Matches(msg, m.keys.Down):
		m.inspectorViewport.ScrollDown(1)
	case key.Matches(msg, m.keys.Up):
		m.inspectorViewport.ScrollUp(1)
	case key.Matches(msg, m.keys.Top):
		m.inspectorViewport.GotoTop()
	case key.Matches(msg, m.keys.Bottom):
		m.inspectorViewport.GotoBottom()
	case key.Matches(msg, m.keys.HalfPageDown):
		m.inspectorViewport.HalfPageDown()
	case key.Matches(msg, m.keys.HalfPageUp):
		m.inspectorViewport.HalfPageUp()
	case key.Matches(msg, m.keys.PageDown):
		m.inspectorViewport.PageDown()
	case key.Matches(msg, m.keys.PageUp):
		m.inspectorViewport.PageUp()
	}
	return m, nil
}

// toggleInspectedEpisodes flips the episode list collapse state for the
// inspected item, starting from its effective (auto-expand) default.
func (m *Model) toggleInspectedEpisodes() {
	item := m.getInspectedItem()
	if item == nil {
		return
	}
	episodes, totals := item.EpisodeSnapshot()
	collapsed := m.isEpisodesCollapsed(*item, episodes, totals)
	m.detailState.episodeCollapsed[item.ID] = !collapsed
	m.updateInspectorViewport()
}

// initInspectorViewport initializes the inspector viewport.
func (m *Model) initInspectorViewport() {
	m.inspectorViewport = viewport.New(
		viewport.WithWidth(m.width),
		viewport.WithHeight(max(m.height-4, 1)),
	)
}

// updateInspectorViewport refreshes the inspector viewport content for the
// active tab. Chrome around it: header, command bar, item line, tab bar.
func (m *Model) updateInspectorViewport() {
	if !m.inspecting {
		return
	}
	if m.inspectorViewport.Width() == 0 {
		m.initInspectorViewport()
	}
	m.inspectorViewport.SetWidth(m.width)
	m.inspectorViewport.SetHeight(max(m.height-4, 1))

	item := m.getInspectedItem()
	if item == nil {
		m.inspectorViewport.SetContent(m.theme.Styles().MutedText.Render("Item no longer in queue"))
		return
	}

	switch m.inspectorTab {
	case tabEpisodes:
		m.inspectorViewport.SetContent(m.renderEpisodesTab(*item))
	case tabProblems:
		m.inspectorViewport.SetContent(m.renderItemProblems(item))
	default:
		m.inspectorViewport.SetContent(m.renderDetailContent(*item, m.width))
	}
}

// renderInspector renders the full inspector: item line, tab bar, content.
func (m Model) renderInspector() string {
	styles := m.theme.Styles()

	var b strings.Builder
	b.WriteString(m.renderInspectorItemLine(styles))
	b.WriteString("\n")
	b.WriteString(m.renderInspectorTabBar(styles))
	b.WriteString("\n")

	if m.inspectorTab == tabLogs {
		b.WriteString(m.logViewport.View())
		b.WriteString("\n")
		b.WriteString(m.renderLogStatus(styles))
	} else {
		b.WriteString(m.inspectorViewport.View())
	}
	return b.String()
}

// renderInspectorItemLine renders the persistent item identity line.
func (m Model) renderInspectorItemLine(styles Styles) string {
	item := m.getInspectedItem()
	if item == nil {
		return styles.MutedText.Render(fmt.Sprintf("Item #%d (gone)", m.inspectedID))
	}

	parts := []string{
		styles.Text.Bold(true).Render(composeTitle(*item)),
		m.renderStatusChips(*item, styles),
		styles.MutedText.Render(fmt.Sprintf("#%d", item.ID)),
	}
	if updated := parseTimestamp(item.UpdatedAt); !updated.IsZero() {
		parts = append(parts, styles.FaintText.Render(humanizeDuration(time.Since(updated))))
	}
	return strings.Join(parts, "  ")
}

// renderInspectorTabBar renders the numbered tab bar.
func (m Model) renderInspectorTabBar(styles Styles) string {
	segments := make([]string, 0, tabCount)
	for i, label := range inspectorTabLabels {
		num := fmt.Sprintf("%d", i+1)
		if inspectorTab(i) == m.inspectorTab {
			segments = append(segments, styles.AccentText.Bold(true).Render(num+" "+label))
		} else {
			segments = append(segments, styles.FaintText.Render(num+" "+label))
		}
	}
	return strings.Join(segments, styles.RuleText.Render("  │  "))
}

// renderEpisodesTab renders the Episodes tab content.
func (m *Model) renderEpisodesTab(item spindle.QueueItem) string {
	styles := m.theme.Styles()
	episodes, totals := item.EpisodeSnapshot()
	if len(episodes) == 0 {
		return styles.MutedText.Render("No episodes for this item")
	}

	var b strings.Builder
	m.renderEpisodeList(&b, item, styles, totals)
	return b.String()
}
