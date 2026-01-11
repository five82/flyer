// Package tea provides a Bubble Tea-based TUI for Flyer.
package ui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/five82/flyer/internal/config"
	"github.com/five82/flyer/internal/prefs"
	"github.com/five82/flyer/internal/spindle"
	"github.com/five82/flyer/internal/state"
)

// View represents the current active view.
type View int

const (
	ViewQueue View = iota
	ViewLogs
	ViewProblems
)

// QueueFilter represents the queue filter mode.
type QueueFilter int

const (
	FilterAll QueueFilter = iota
	FilterFailed
	FilterReview
	FilterProcessing
)

// detailState holds per-item detail view state.
type detailState struct {
	episodeCollapsed map[int64]bool
	pathExpanded     map[int64]bool
}

// Options configures the UI.
type Options struct {
	Context   context.Context
	Client    *spindle.Client
	Store     *state.Store
	Config    *config.Config
	PollTick  time.Duration
	ThemeName string
	PrefsPath string
}

// Model is the root application state for Bubble Tea.
type Model struct {
	// Configuration
	ctx       context.Context
	client    *spindle.Client
	store     *state.Store
	config    *config.Config
	prefsPath string
	pollTick  time.Duration

	// UI state
	theme       Theme
	currentView View
	width       int
	height      int
	ready       bool
	focusedPane int // 0 = table, 1 = detail

	// Data state
	snapshot    state.Snapshot
	lastUpdated time.Time

	// Queue state
	selectedRow int
	filterMode  QueueFilter

	// Detail state
	detailViewport viewport.Model
	detailState    detailState

	// Log state
	logViewport viewport.Model
	logState    logState

	// Problems state
	problemsViewport viewport.Model
	problemsState    problemsState

	// Help overlay
	showHelp bool

	// Log filters modal
	showLogFilters    bool
	logFilterInputs   [3]textinput.Model // component, lane, request
	logFilterFocusIdx int
}

// New creates a new Bubble Tea model.
func New(opts Options) Model {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	pollTick := opts.PollTick
	if pollTick == 0 {
		pollTick = time.Second
	}

	themeName := opts.ThemeName
	if themeName == "" {
		themeName = "Dracula"
	}

	prefsPath := opts.PrefsPath
	if prefsPath == "" {
		prefsPath = prefs.DefaultPath()
	}

	return Model{
		ctx:         ctx,
		client:      opts.Client,
		store:       opts.Store,
		config:      opts.Config,
		prefsPath:   prefsPath,
		pollTick:    pollTick,
		theme:       GetTheme(themeName),
		currentView: ViewQueue,
		detailState: detailState{
			episodeCollapsed: make(map[int64]bool),
			pathExpanded:     make(map[int64]bool),
		},
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.EnterAltScreen,
		tickCmd(m.pollTick),
	}
	// Fetch snapshot immediately on start
	if m.store != nil {
		cmds = append(cmds, fetchSnapshotCmd(m.store))
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.initDetailViewport()
			m.initLogState()
			m.initLogViewport()
			m.initProblemsViewport()
			m.initLogFilterInputs()
		}
		m.ready = true
		m.updateQueueTable()
		m.updateDetailViewport()
		m.updateLogViewport()
		m.updateProblemsViewport()
		return m, nil

	case tickMsg:
		return m.handleTick()

	case snapshotMsg:
		m.snapshot = state.Snapshot(msg)
		m.lastUpdated = time.Now()
		m.updateQueueTable()
		m.updateDetailViewport()
		m.updateProblemsViewport()
		return m, nil

	case logBatchMsg:
		m.handleLogBatch(msg)
		return m, nil

	case logTailMsg:
		m.handleLogTail(msg)
		return m, nil

	case logErrorMsg:
		// Log errors are handled silently for now
		return m, nil

	case problemsLogBatchMsg:
		m.handleProblemsLogBatch(msg)
		return m, nil

	case problemsLogErrorMsg:
		// Problems log errors are handled silently for now
		return m, nil
	}

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Show help overlay if active
	if m.showHelp {
		return m.renderHelp()
	}

	// Show log filters modal if active
	if m.showLogFilters {
		return m.renderLogFilters()
	}

	return m.renderMain()
}

// handleKey processes keyboard input (matching tview bindings).
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle help overlay
	if m.showHelp {
		// Any key closes help
		m.showHelp = false
		return m, nil
	}

	// Handle log filters modal
	if m.showLogFilters {
		return m.handleLogFiltersKey(msg)
	}

	// Global keys (matching tview)
	switch msg.String() {
	case "ctrl+c", "e":
		// e = exit (matching tview)
		return m, tea.Quit

	case "h", "?":
		m.showHelp = true
		return m, nil

	case "T":
		// Cycle theme
		m.theme = GetTheme(NextTheme(m.theme.Name))
		if m.prefsPath != "" {
			_ = prefs.Save(m.prefsPath, prefs.Prefs{Theme: m.theme.Name})
		}
		return m, nil

	case "tab":
		// Toggle focus between table and detail pane (matching tview)
		m.toggleFocus()
		if m.currentView == ViewLogs {
			return m, m.refreshLogs() // Fetch immediately when entering logs
		}
		return m, nil

	case "shift+tab":
		// Toggle focus reverse
		m.toggleFocusReverse()
		if m.currentView == ViewLogs {
			return m, m.refreshLogs() // Fetch immediately when entering logs
		}
		return m, nil

	case "q":
		// Go to queue view (matching tview)
		m.currentView = ViewQueue
		return m, nil

	case "l":
		// Daemon logs
		m.logState.mode = logSourceDaemon
		m.currentView = ViewLogs
		return m, m.refreshLogs() // Fetch immediately

	case "i":
		// Item logs
		m.logState.mode = logSourceItem
		m.currentView = ViewLogs
		return m, m.refreshLogs() // Fetch immediately

	case "p":
		m.currentView = ViewProblems
		return m, nil

	case "t":
		// Toggle episodes collapsed (for current item)
		m.toggleEpisodesCollapsed()
		return m, nil

	case "P":
		// Toggle path detail (for current item)
		m.togglePathExpanded()
		return m, nil

	case "f":
		// Cycle queue filter
		m.cycleFilter()
		return m, nil

	case "esc":
		m.currentView = ViewQueue
		return m, nil
	}

	// View-specific keys
	switch m.currentView {
	case ViewQueue:
		return m.handleQueueKey(msg)
	case ViewLogs:
		return m.handleLogsKey(msg)
	case ViewProblems:
		return m.handleProblemsKey(msg)
	}

	return m, nil
}

// toggleFocus cycles focus forward through views (matching tview).
// Cycle: Queue(table) → Queue(detail) → Item Logs → Problems → Queue(table)
func (m *Model) toggleFocus() {
	switch m.currentView {
	case ViewQueue:
		if m.focusedPane == 0 {
			// Table focused → focus detail pane
			m.focusedPane = 1
		} else {
			// Detail focused → go to item logs
			m.logState.mode = logSourceItem
			m.currentView = ViewLogs
			m.focusedPane = 0
		}
	case ViewLogs:
		// Logs → go to problems
		m.currentView = ViewProblems
	case ViewProblems:
		// Problems → back to queue (table focused)
		m.currentView = ViewQueue
		m.focusedPane = 0
	}
}

// toggleEpisodesCollapsed toggles episode list visibility for current item.
func (m *Model) toggleEpisodesCollapsed() {
	item := m.getSelectedItem()
	if item == nil {
		return
	}
	current := m.detailState.episodeCollapsed[item.ID]
	m.detailState.episodeCollapsed[item.ID] = !current
	m.updateDetailViewport()
}

// togglePathExpanded toggles path detail visibility for current item.
func (m *Model) togglePathExpanded() {
	item := m.getSelectedItem()
	if item == nil {
		return
	}
	current := m.detailState.pathExpanded[item.ID]
	m.detailState.pathExpanded[item.ID] = !current
	m.updateDetailViewport()
}

// cycleFilter cycles through queue filter modes.
func (m *Model) cycleFilter() {
	switch m.filterMode {
	case FilterAll:
		m.filterMode = FilterFailed
	case FilterFailed:
		m.filterMode = FilterReview
	case FilterReview:
		m.filterMode = FilterProcessing
	default:
		m.filterMode = FilterAll
	}
}

// filterLabel returns the display label for the current filter mode.
func (m *Model) filterLabel() string {
	switch m.filterMode {
	case FilterFailed:
		return "Failed"
	case FilterReview:
		return "Review"
	case FilterProcessing:
		return "Active"
	default:
		return "All"
	}
}

// toggleFocusReverse cycles focus backward through views (matching tview).
func (m *Model) toggleFocusReverse() {
	switch m.currentView {
	case ViewQueue:
		if m.focusedPane == 1 {
			// Detail focused → focus table
			m.focusedPane = 0
		} else {
			// Table focused → go to problems
			m.currentView = ViewProblems
		}
	case ViewLogs:
		// Logs → go to queue with detail focus
		m.currentView = ViewQueue
		m.focusedPane = 1
	case ViewProblems:
		// Problems → go to item logs
		m.logState.mode = logSourceItem
		m.currentView = ViewLogs
	}
}

// handleQueueKey processes keyboard input for queue view.
func (m Model) handleQueueKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.getSortedItems()
	itemCount := len(items)
	if itemCount == 0 {
		return m, nil
	}

	switch msg.String() {
	case "j", "down":
		if m.selectedRow < itemCount-1 {
			m.selectedRow++
		}
	case "k", "up":
		if m.selectedRow > 0 {
			m.selectedRow--
		}
	case "g", "home":
		m.selectedRow = 0
	case "G", "end":
		m.selectedRow = itemCount - 1
	}

	return m, nil
}

// handleTick processes the polling tick.
func (m Model) handleTick() (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Fetch latest snapshot
	if m.store != nil {
		cmds = append(cmds, fetchSnapshotCmd(m.store))
	}

	// Refresh logs if in log view and following
	if m.currentView == ViewLogs && m.logState.follow {
		if cmd := m.refreshLogs(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Refresh problems logs if in problems view
	if m.currentView == ViewProblems {
		if cmd := m.refreshProblemsLogs(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Schedule next tick
	cmds = append(cmds, tickCmd(m.pollTick))

	return m, tea.Batch(cmds...)
}

// renderMain renders the full UI (matching tview layout).
func (m Model) renderMain() string {
	var b strings.Builder

	// Header line 1: logo + status
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Header line 2: command bar
	b.WriteString(m.renderCommandBar())
	b.WriteString("\n")

	// Main content
	b.WriteString(m.renderContent())

	return b.String()
}

// renderContent renders the main content area based on current view.
func (m Model) renderContent() string {
	switch m.currentView {
	case ViewQueue:
		return m.renderQueue()
	case ViewLogs:
		return m.renderLogs()
	case ViewProblems:
		return m.renderProblems()
	default:
		return ""
	}
}

// Messages

type tickMsg time.Time

type snapshotMsg state.Snapshot

// Commands

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchSnapshotCmd(store *state.Store) tea.Cmd {
	return func() tea.Msg {
		return snapshotMsg(store.Snapshot())
	}
}

// Run starts the Bubble Tea program.
func Run(opts Options) error {
	m := New(opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
