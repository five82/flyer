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

type logPreviewEntry struct {
	text   string
	readAt time.Time
}

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
	table          *tview.Table
	detail         *tview.TextView
	logView        *tview.TextView
	problemTable   *tview.Table
	problemSummary *tview.TextView
	problemBar     *tview.TextView
	problemDrawer  *tview.Flex
	mainLayout     *tview.Flex
	theme          Theme

	// Search state
	searchInput        *tview.InputField
	searchStatus       *tview.TextView
	searchRegex        *regexp.Regexp
	searchMatches      []int
	currentSearchMatch int
	lastSearchPattern  string
	searchMode         bool

	// Data and navigation state
	items        []spindle.QueueItem
	displayItems []spindle.QueueItem
	currentView  string // "queue", "detail", "logs"
	filterMode   queueFilter
	lastDetailID int64

	// Log viewing state
	logMode          logSource
	lastLogPath      string
	lastLogKey       string
	lastLogSet       time.Time
	rawLogLines      []string
	logCursor        map[string]uint64
	currentItemLogID int64

	// Problems drawer state
	problemEntries   []problemEntry
	problemShortcuts map[rune]int64
	problemsOpen     bool

	// Detail view state
	episodeCollapsed map[int64]bool
	pathExpanded     map[int64]bool
	logPreviewCache  map[string]logPreviewEntry
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

	problemBar := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	problemBar.SetBackgroundColor(hexToColor(theme.Problems.BarBackground))
	problemBar.SetTextColor(hexToColor(theme.Problems.BarText))

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
		problemBar:       problemBar,
		problemDrawer:    problemDrawer,
		currentView:      "queue",
		problemShortcuts: map[rune]int64{},
		theme:            theme,
		logCursor:        make(map[string]uint64),
		episodeCollapsed: map[int64]bool{},
		pathExpanded:     map[int64]bool{},
		logPreviewCache:  map[string]logPreviewEntry{},
	}

	vm.table.SetSelectedFunc(func(row, column int) {
		vm.updateDetail(row)
	})
	vm.table.SetSelectionChangedFunc(func(row, column int) {
		vm.updateDetail(row)
	})

	vm.table.SetFocusFunc(func() {
		vm.table.SetBorderColor(vm.theme.TableBorderFocusColor())
		vm.detail.SetBorderColor(vm.theme.TableBorderColor())
		vm.logView.SetBorderColor(vm.theme.TableBorderColor())
	})

	vm.detail.SetFocusFunc(func() {
		vm.table.SetBorderColor(vm.theme.TableBorderColor())
		vm.detail.SetBorderColor(vm.theme.TableBorderFocusColor())
		vm.logView.SetBorderColor(vm.theme.TableBorderColor())
	})

	vm.logView.SetFocusFunc(func() {
		vm.table.SetBorderColor(vm.theme.TableBorderColor())
		vm.detail.SetBorderColor(vm.theme.TableBorderColor())
		vm.logView.SetBorderColor(vm.theme.TableBorderFocusColor())
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
		AddItem(vm.logoView, 8, 0, false).
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
	queuePane := tview.NewFlex().SetDirection(tview.FlexColumn)
	queuePane.SetBackgroundColor(vm.theme.SurfaceColor())
	queuePane.
		AddItem(vm.table, 0, 40, true).
		AddItem(vm.detail, 0, 60, false)

	vm.mainContent = tview.NewPages()
	vm.mainContent.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.mainContent.AddPage("queue", queuePane, true, true)
	vm.mainContent.AddPage("logs", logContainer, true, false)

	// Main layout: header + content + optional problems drawer
	vm.mainLayout = tview.NewFlex().SetDirection(tview.FlexRow)
	vm.mainLayout.SetBackgroundColor(vm.theme.BackgroundColor())
	vm.mainLayout.
		AddItem(vm.header, 3, 0, false). // keep header to ~2-3 rows max
		AddItem(vm.problemBar, 1, 0, false).
		AddItem(vm.mainContent, 0, 1, true).
		AddItem(vm.problemDrawer, 0, 0, false) // height managed dynamically

	// Start with the problem surfaces hidden.
	vm.mainLayout.ResizeItem(vm.problemBar, 0, 0)
	vm.mainLayout.ResizeItem(vm.problemDrawer, 0, 0)

	return vm.mainLayout
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
	commands := []cmd{}
	switch view {
	case "logs":
		commands = []cmd{
			{"<l>", "Cycle Logs"},
			{"<Tab>", "Switch Pane"},
			{"</>", "Search"},
			{"<n>/<N>", "Next/Prev"},
			{"<p>", "Problems"},
			{"<q>", "Queue"},
			{"<h>", "Help"},
			{"<e>", "Exit"},
		}
	case "detail":
		episodesLabel := "Episodes: expand"
		pathLabel := "Paths: full"
		if item := vm.selectedItem(); item != nil {
			if !vm.episodesCollapsed(item.ID) {
				episodesLabel = "Episodes: collapse"
			}
			if vm.pathsExpanded(item.ID) {
				pathLabel = "Paths: compact"
			}
		}
		commands = []cmd{
			{"<q>", "Queue"},
			{"<l>", "Logs"},
			{"<i>", "Item Log"},
			{"<t>", episodesLabel},
			{"<P>", pathLabel},
			{"<Tab>", "Switch Pane"},
			{"<f>", "Filter"},
			{"<p>", "Problems"},
			{"<h>", "Help"},
			{"<e>", "Exit"},
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
		commands = []cmd{
			{"<l>", "Logs"},
			{"<i>", "Item Log"},
			{"<f>", "Filter: " + filterLabel},
			{"<p>", "Problems"},
			{"<Tab>", "Switch Pane"},
			{"<h>", "Help"},
			{"<e>", "Exit"},
		}
	}

	segments := make([]string, 0, len(commands))
	for _, cmd := range commands {
		segments = append(segments, fmt.Sprintf("[%s]%s[-] [%s]%s[-]", vm.theme.Text.AccentSoft, cmd.key, vm.theme.Text.Faint, cmd.desc))
	}
	vm.cmdBar.SetText(strings.Join(segments, "  •  "))
}
