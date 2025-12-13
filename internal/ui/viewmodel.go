package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

type viewModel struct {
	// Core application state
	app     *tview.Application
	ctx     context.Context
	client  *spindle.Client
	options Options
	root    *tview.Pages

	// Header components
	header      *tview.Flex
	statusView  *tview.TextView
	cmdBar      *tview.TextView
	logoView    *tview.TextView
	lastRefresh time.Time

	// Main content views
	mainContent    *tview.Pages
	queuePane      *tview.Flex
	table          *tview.Table
	detail         *tview.TextView
	logView        *tview.TextView
	problemTable   *tview.Table
	problemSummary *tview.TextView
	problemDrawer  *tview.Flex
	mainLayout     *tview.Flex
	theme          Theme
	queueLayout    string // "wide" or "stacked"

	// Search state
	searchInput        *tview.InputField
	searchHint         *tview.TextView
	searchStatus       *tview.TextView
	searchRegex        *regexp.Regexp
	searchMatches      []int
	currentSearchMatch int
	lastSearchPattern  string
	searchMode         bool

	// Queue search state
	queueSearchInput   *tview.InputField
	queueSearchHint    *tview.TextView
	queueSearchRegex   *regexp.Regexp
	queueSearchPattern string
	queueSearchMode    bool

	// Data and navigation state
	items        []spindle.QueueItem
	displayItems []spindle.QueueItem
	currentView  string // "queue", "detail", "logs"
	filterMode   queueFilter
	lastDetailID int64

	// Log viewing state
	logMode            logSource
	logFollow          bool
	lastLogPath        string
	lastLogKey         string
	lastLogFileKey     string
	lastLogSet         time.Time
	rawLogLines        []string
	logCursor          map[string]uint64
	logFileCursor      map[string]int64
	currentItemLogID   int64
	logFilterComponent string
	logFilterLane      string
	logFilterRequest   string

	// Problems drawer state
	problemEntries   []problemEntry
	problemShortcuts map[rune]int64
	problemsOpen     bool

	// Detail view state
	episodeCollapsed map[int64]bool
	pathExpanded     map[int64]bool
	fullscreenMode   bool
}

func newViewModel(app *tview.Application, opts Options) *viewModel {
	theme := defaultTheme()

	// Override focus borders to use single lines instead of double lines
	tview.Borders.HorizontalFocus = tview.Borders.Horizontal
	tview.Borders.VerticalFocus = tview.Borders.Vertical
	tview.Borders.TopLeftFocus = tview.Borders.TopLeft
	tview.Borders.TopRightFocus = tview.Borders.TopRight
	tview.Borders.BottomLeftFocus = tview.Borders.BottomLeft
	tview.Borders.BottomRightFocus = tview.Borders.BottomRight

	tview.Styles.PrimitiveBackgroundColor = theme.BackgroundColor()
	tview.Styles.ContrastBackgroundColor = theme.SurfaceColor()
	tview.Styles.MoreContrastBackgroundColor = theme.SurfaceAltColor()
	tview.Styles.PrimaryTextColor = tcell.ColorDefault

	statusView := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	statusView.SetTextAlign(tview.AlignLeft)
	statusView.SetBackgroundColor(theme.SurfaceColor())
	statusView.SetTextColor(hexToColor(theme.Text.Primary))

	cmdBar := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	cmdBar.SetBackgroundColor(theme.SurfaceColor())
	cmdBar.SetTextAlign(tview.AlignLeft)
	cmdBar.SetTextColor(hexToColor(theme.Text.Secondary))

	logoView := tview.NewTextView()
	logoView.SetTextAlign(tview.AlignLeft)
	logoView.SetDynamicColors(true)
	logoView.SetRegions(true)
	logoView.SetBackgroundColor(theme.SurfaceColor())
	logoView.SetText(createLogo(theme))

	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" [::b]Queue[::-] ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)
	table.SetBackgroundColor(theme.SurfaceColor())
	table.SetBorderColor(theme.TableBorderColor())
	table.SetSelectedStyle(tcell.StyleDefault.Background(theme.TableSelectionBackground()).Foreground(theme.TableSelectionTextColor()))

	detail := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	detail.SetBorder(true).SetTitle(" [::b]Details[::-] ")
	detail.SetBackgroundColor(theme.SurfaceAltColor())
	detail.SetBorderColor(theme.TableBorderColor())

	logView := tview.NewTextView().SetDynamicColors(true)
	logView.SetBorder(true).SetTitle(" [::b]Daemon Log[::-] ")
	logView.SetBackgroundColor(theme.SurfaceAltColor())
	logView.SetBorderColor(theme.TableBorderColor())
	logView.ScrollToEnd()

	searchStatus := tview.NewTextView().SetDynamicColors(true)
	searchStatus.SetBackgroundColor(theme.SurfaceColor())
	searchStatus.SetTextColor(hexToColor(theme.Text.Secondary))

	problemsTable := tview.NewTable()
	problemsTable.SetBorder(true)
	problemsTable.SetTitle(" [::b]Problems[::-] ")
	problemsTable.SetBorderColor(theme.ProblemBorderColor())
	problemsTable.SetBackgroundColor(theme.SurfaceColor())
	problemsTable.SetSelectable(false, false)
	problemsTable.SetFixed(1, 0)

	problemSummary := tview.NewTextView().SetDynamicColors(true)
	problemSummary.SetBackgroundColor(theme.SurfaceColor())
	problemSummary.SetTextColor(hexToColor(theme.Text.Primary))
	problemSummary.SetWrap(false)

	problemDrawer := tview.NewFlex().SetDirection(tview.FlexRow)
	problemDrawer.SetBackgroundColor(theme.SurfaceColor())
	problemDrawer.
		AddItem(problemsTable, 0, 1, false).
		AddItem(problemSummary, 1, 0, false)

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	vm := &viewModel{
		app:              app,
		ctx:              ctx,
		client:           opts.Client,
		options:          opts,
		statusView:       statusView,
		cmdBar:           cmdBar,
		logoView:         logoView,
		table:            table,
		detail:           detail,
		logView:          logView,
		searchStatus:     searchStatus,
		problemTable:     problemsTable,
		problemSummary:   problemSummary,
		problemDrawer:    problemDrawer,
		currentView:      "queue",
		problemShortcuts: map[rune]int64{},
		theme:            theme,
		logFollow:        true,
		logCursor:        make(map[string]uint64),
		logFileCursor:    make(map[string]int64),
		episodeCollapsed: map[int64]bool{},
		pathExpanded:     map[int64]bool{},
	}

	vm.logView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyUp || event.Key() == tcell.KeyPgUp || event.Key() == tcell.KeyHome:
			if vm.logFollow {
				vm.logFollow = false
				vm.updateLogStatus(false, vm.lastLogPath)
				vm.setCommandBar("logs")
			}
			return event
		case event.Rune() == 'k':
			// Vim-style up
			if vm.logFollow {
				vm.logFollow = false
				vm.updateLogStatus(false, vm.lastLogPath)
				vm.setCommandBar("logs")
			}
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == 'j':
			// Vim-style down
			if vm.logFollow {
				vm.logFollow = false
				vm.updateLogStatus(false, vm.lastLogPath)
				vm.setCommandBar("logs")
			}
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'g':
			// Vim-style top (gg)
			if vm.logFollow {
				vm.logFollow = false
				vm.updateLogStatus(false, vm.lastLogPath)
				vm.setCommandBar("logs")
			}
			return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
		case event.Key() == tcell.KeyEnd || event.Rune() == 'G':
			vm.logFollow = true
			vm.logView.ScrollToEnd()
			vm.refreshLogs(true)
			vm.updateLogStatus(vm.logFollow, vm.lastLogPath)
			vm.setCommandBar("logs")
			return nil
		case event.Rune() == ' ':
			vm.logFollow = !vm.logFollow
			if vm.logFollow {
				vm.logView.ScrollToEnd()
				vm.refreshLogs(true)
			}
			vm.updateLogStatus(vm.logFollow, vm.lastLogPath)
			vm.setCommandBar("logs")
			return nil
		}
		return event
	})

	vm.detail.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			// Vim-style down
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			// Vim-style up
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case 'g':
			// Vim-style top (gg)
			return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
		case 'G':
			// Vim-style bottom
			return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
		}
		return event
	})

	vm.table.SetSelectedFunc(func(row, column int) {
		vm.updateDetail(row)
	})
	vm.table.SetSelectionChangedFunc(func(row, column int) {
		vm.updateDetail(row)
	})

	vm.table.SetFocusFunc(func() {
		vm.table.SetBorderColor(vm.theme.TableBorderFocusColor())
		vm.table.SetBackgroundColor(vm.theme.FocusBackgroundColor())
		vm.detail.SetBorderColor(vm.theme.TableBorderColor())
		vm.detail.SetBackgroundColor(vm.theme.SurfaceAltColor())
		vm.logView.SetBorderColor(vm.theme.TableBorderColor())
		vm.logView.SetBackgroundColor(vm.theme.SurfaceAltColor())
		vm.setCommandBar("queue")
	})

	vm.detail.SetFocusFunc(func() {
		vm.table.SetBorderColor(vm.theme.TableBorderColor())
		vm.table.SetBackgroundColor(vm.theme.SurfaceColor())
		vm.detail.SetBorderColor(vm.theme.TableBorderFocusColor())
		vm.detail.SetBackgroundColor(vm.theme.FocusBackgroundColor())
		vm.logView.SetBorderColor(vm.theme.TableBorderColor())
		vm.logView.SetBackgroundColor(vm.theme.SurfaceAltColor())
		vm.setCommandBar("detail")
	})

	vm.logView.SetFocusFunc(func() {
		vm.table.SetBorderColor(vm.theme.TableBorderColor())
		vm.table.SetBackgroundColor(vm.theme.SurfaceColor())
		vm.detail.SetBorderColor(vm.theme.TableBorderColor())
		vm.detail.SetBackgroundColor(vm.theme.SurfaceAltColor())
		vm.logView.SetBorderColor(vm.theme.TableBorderFocusColor())
		vm.logView.SetBackgroundColor(vm.theme.FocusBackgroundColor())
		vm.setCommandBar("logs")
	})

	vm.root = tview.NewPages()
	vm.root.SetBackgroundColor(theme.BackgroundColor())
	vm.root.AddPage("main", vm.buildMainLayout(), true, true)

	app.SetRoot(vm.root, true)
	app.SetFocus(vm.table)
	vm.setCommandBar("queue")

	return vm
}

func (vm *viewModel) buildMainLayout() tview.Primitive {
	// Header: dense two-line bar (stats + commands) with compact logo
	headerTop := tview.NewFlex().SetDirection(tview.FlexColumn)
	headerTop.SetBackgroundColor(vm.theme.SurfaceColor())
	headerTop.
		AddItem(vm.logoView, 6, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(vm.statusView, 0, 1, false)

	vm.header = tview.NewFlex().SetDirection(tview.FlexRow)
	vm.header.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.header.
		AddItem(headerTop, 0, 1, false).
		AddItem(vm.cmdBar, 1, 0, false)

	// Create log view container with search status
	logContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	logContainer.SetBackgroundColor(vm.theme.SurfaceColor())
	logContainer.
		AddItem(vm.logView, 0, 1, true).      // Main log content
		AddItem(vm.searchStatus, 1, 0, false) // Search status bar at bottom

	// Create main content pages for different views
	// Dual-pane queue view: table + detail side-by-side
	vm.queuePane = tview.NewFlex().SetDirection(tview.FlexColumn)
	vm.queuePane.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.applyQueueLayout("wide")

	vm.mainContent = tview.NewPages()
	vm.mainContent.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.mainContent.AddPage("queue", vm.queuePane, true, true)
	vm.mainContent.AddPage("detail-fullscreen", vm.detail, true, false)
	vm.mainContent.AddPage("logs", logContainer, true, false)

	// Main layout: header + content + optional problems drawer
	vm.mainLayout = tview.NewFlex().SetDirection(tview.FlexRow)
	vm.mainLayout.SetBackgroundColor(vm.theme.BackgroundColor())
	vm.mainLayout.
		AddItem(vm.header, 2, 0, false). // keep header compact; status is one line + command bar
		AddItem(vm.mainContent, 0, 1, true).
		AddItem(vm.problemDrawer, 0, 0, false) // height managed dynamically

	// Start with the problems drawer hidden.
	vm.mainLayout.ResizeItem(vm.problemDrawer, 0, 0)

	return vm.mainLayout
}

func (vm *viewModel) applyQueueLayout(layout string) {
	if vm.queuePane == nil {
		return
	}
	switch layout {
	case "stacked":
		vm.queuePane.Clear()
		vm.queuePane.SetDirection(tview.FlexRow)
		vm.queuePane.AddItem(vm.table, 0, 3, true)
		vm.queuePane.AddItem(vm.detail, 0, 2, false)
		vm.queueLayout = "stacked"
	default:
		vm.queuePane.Clear()
		vm.queuePane.SetDirection(tview.FlexColumn)
		vm.queuePane.AddItem(vm.table, 0, 40, true)
		vm.queuePane.AddItem(vm.detail, 0, 60, false)
		vm.queueLayout = "wide"
	}
}

func (vm *viewModel) maybeUpdateQueueLayout() {
	if vm == nil || vm.app == nil || vm.queuePane == nil {
		return
	}

	width := 0
	if vm.mainLayout != nil {
		_, _, w, _ := vm.mainLayout.GetRect()
		width = w
	}
	if width <= 0 {
		_, _, w, _ := vm.queuePane.GetRect()
		width = w
	}

	target := "wide"
	if width > 0 && width < 120 {
		target = "stacked"
	}
	if target == vm.queueLayout {
		return
	}
	vm.applyQueueLayout(target)
}

func (vm *viewModel) ensureSelection() {
	rows := vm.table.GetRowCount() - 1 // excludes header
	if rows == 0 {
		vm.table.Select(0, 0)
		vm.detail.SetText(fmt.Sprintf("[%s]Queue is empty.[-]\nAdd discs to Spindle or check logs at:\n[%s]%s[-]",
			vm.theme.Text.Muted,
			vm.theme.Text.Accent,
			tview.Escape(vm.options.Config.DaemonLogPath())))
		return
	}
	row, _ := vm.table.GetSelection()
	if row <= 0 {
		vm.table.Select(1, 0)
	}
	if row > rows {
		vm.table.Select(rows, 0)
	}
}

func (vm *viewModel) setCommandBar(view string) {
	type cmd struct{ key, desc string }

	_, _, width, _ := vm.cmdBar.GetInnerRect()
	if width <= 0 {
		width = 120
	}
	compact := width < 110

	var commands []cmd
	switch view {
	case "logs":
		followLabel := "Pause"
		if !vm.logFollow {
			followLabel = "Follow"
		}
		filterLabel := "Filter"
		if vm.logFiltersActive() {
			filterLabel = "Filter:on"
		}
		if compact {
			commands = []cmd{
				{"<Tab>", "Pane"},
				{"<Space>", followLabel},
				{"</>", "Search"},
				{"<n/N>", "Next"},
				{"<l>", "Source"},
				{"<F>", filterLabel},
				{"<q>", "Queue"},
				{"<?>", "Help"},
			}
		} else {
			commands = []cmd{
				{"<Tab>", "Switch Pane"},
				{"<Space>", followLabel},
				{"</>", "Search"},
				{"<n>/<N>", "Next/Prev"},
				{"<l>", "Rotate Source"},
				{"<F>", "Filters"},
				{"<q>", "Queue"},
				{"<?>", "Help"},
			}
		}
	case "detail":
		if compact {
			fullscreenLabel := "Full"
			if vm.fullscreenMode {
				fullscreenLabel = "Split"
			}
			commands = []cmd{
				{"<Tab>", "Pane"},
				{"<Enter>", fullscreenLabel},
				{"</>", "Search"},
				{"<q>", "Queue"},
				{"<l>", "Logs"},
				{"<p>", "Problems"},
				{"<?>", "Help"},
			}
		} else {
			episodesLabel := "Episodes: expand"
			pathLabel := "Paths: full"
			fullscreenLabel := "Fullscreen"
			if vm.fullscreenMode {
				fullscreenLabel = "Split View"
			}
			showPaths := false
			if item := vm.selectedItem(); item != nil {
				if !vm.episodesCollapsed(item.ID) {
					episodesLabel = "Episodes: collapse"
				}
				if vm.pathsExpanded(item.ID) {
					pathLabel = "Paths: compact"
				}
				showPaths = strings.TrimSpace(item.SourcePath) != "" ||
					strings.TrimSpace(item.BackgroundLogPath) != ""
			}
			commands = []cmd{
				{"<Tab>", "Switch Pane"},
				{"<Enter>", fullscreenLabel},
				{"</>", "Search"},
				{"<q>", "Queue"},
				{"<l>", "Logs"},
				{"<t>", episodesLabel},
			}
			if showPaths {
				commands = append(commands, cmd{"<P>", pathLabel})
			}
			commands = append(commands,
				cmd{"<p>", "Problems"},
				cmd{"<?>", "Help"},
			)
		}
	default:
		filterLabel := "All"
		switch vm.filterMode {
		case filterFailed:
			filterLabel = "Failed"
		case filterReview:
			filterLabel = "Review"
		case filterProcessing:
			filterLabel = "Processing"
		}
		if compact {
			commands = []cmd{
				{"<Tab>", "Pane"},
				{"</>", "Search"},
				{"<l>", "Logs"},
				{"<f>", "Filter: " + filterLabel},
				{"<p>", "Problems"},
				{"<?>", "Help"},
			}
		} else {
			commands = []cmd{
				{"<Tab>", "Switch Pane"},
				{"</>", "Search"},
				{"<l>", "Logs"},
				{"<i>", "Item Logs"},
				{"<f>", "Filter: " + filterLabel},
				{"<p>", "Problems"},
				{"<?>", "Help"},
			}
		}
	}

	segments := make([]string, 0, len(commands))
	for _, cmd := range commands {
		segments = append(segments, fmt.Sprintf("[%s]%s[-] [%s]%s[-]", vm.theme.Text.AccentSoft, cmd.key, vm.theme.Text.Faint, cmd.desc))
	}
	if view != "logs" && vm.queueSearchPattern != "" {
		pattern := truncate(vm.queueSearchPattern, 18)
		segments = append(segments, fmt.Sprintf("[%s]search[-] [%s]/%s[-]", vm.theme.Text.Faint, vm.theme.Text.Accent, tview.Escape(pattern)))
	}
	if view == "logs" && vm.lastSearchPattern != "" {
		pattern := truncate(vm.lastSearchPattern, 18)
		matchInfo := ""
		if len(vm.searchMatches) > 0 {
			matchInfo = fmt.Sprintf(" [%s](%d matches)[-]", vm.theme.Text.Muted, len(vm.searchMatches))
		}
		segments = append(segments, fmt.Sprintf("[%s]search[-] [%s]/%s[-]%s", vm.theme.Text.Faint, vm.theme.Text.Accent, tview.Escape(pattern), matchInfo))
	}
	separator := "  •  "
	if compact {
		separator = "   "
	}

	pane := strings.ToUpper(strings.TrimSpace(view))
	if pane == "" {
		pane = "QUEUE"
	}
	prefix := fmt.Sprintf("[%s::b]%s[-] ", vm.theme.Text.Accent, pane)
	vm.cmdBar.SetText(prefix + strings.Join(segments, separator))
}
