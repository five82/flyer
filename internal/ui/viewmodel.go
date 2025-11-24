package ui

import (
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

	// Log viewing state
	logMode     logSource
	lastLogPath string
	lastLogSet  time.Time
	rawLogLines []string

	// Problems drawer state
	problemEntries   []problemEntry
	problemShortcuts map[rune]int64
	problemsOpen     bool
}

func newViewModel(app *tview.Application, opts Options) *viewModel {
	// Override focus borders to use single lines instead of double lines
	tview.Borders.HorizontalFocus = tview.Borders.Horizontal
	tview.Borders.VerticalFocus = tview.Borders.Vertical
	tview.Borders.TopLeftFocus = tview.Borders.TopLeft
	tview.Borders.TopRightFocus = tview.Borders.TopRight
	tview.Borders.BottomLeftFocus = tview.Borders.BottomLeft
	tview.Borders.BottomRightFocus = tview.Borders.BottomRight

	// Set default focus colors to be less intrusive
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorBlack
	tview.Styles.ContrastBackgroundColor = tcell.ColorBlack
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorBlack
	tview.Styles.PrimaryTextColor = tcell.ColorDefault // Allow dynamic colors

	// Header components (compact toolbar)
	statusView := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	statusView.SetTextAlign(tview.AlignLeft)
	statusView.SetBackgroundColor(tcell.ColorBlack)
	statusView.SetTextColor(tcell.ColorWhite) // Default to white

	// Commands section as a single-line toolbar
	cmdBar := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	cmdBar.SetBackgroundColor(tcell.ColorBlack)
	cmdBar.SetTextAlign(tview.AlignLeft)

	logoView := tview.NewTextView()
	logoView.SetTextAlign(tview.AlignLeft)
	logoView.SetDynamicColors(true)
	logoView.SetRegions(true)
	logoView.SetBackgroundColor(tcell.ColorBlack)
	logoView.SetText(createLogo())

	// Main content components (k9s-style)
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" [::b]Queue[::-] ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)
	table.SetBackgroundColor(tcell.ColorBlack)
	// k9s-style border color
	table.SetBorderColor(tcell.ColorSlateGray)

	detail := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	detail.SetBorder(true).SetTitle(" [::b]Details[::-] ")
	detail.SetBackgroundColor(tcell.ColorBlack)
	detail.SetBorderColor(tcell.ColorSlateGray)

	logView := tview.NewTextView().SetDynamicColors(true)
	logView.SetBorder(true).SetTitle(" [::b]Daemon Log[::-] ")
	logView.SetBackgroundColor(tcell.ColorBlack)
	logView.SetBorderColor(tcell.ColorSlateGray)
	logView.ScrollToEnd()

	// Search status bar (vim-style at bottom)
	searchStatus := tview.NewTextView().SetDynamicColors(true)
	searchStatus.SetBackgroundColor(tcell.ColorBlack)
	searchStatus.SetTextColor(tcell.ColorWhite)

	// Problems drawer components
	problemsTable := tview.NewTable()
	problemsTable.SetBorder(true)
	problemsTable.SetTitle(" [::b]Problems[::-] ")
	problemsTable.SetBorderColor(tcell.ColorIndianRed)
	problemsTable.SetBackgroundColor(tcell.ColorBlack)
	problemsTable.SetSelectable(false, false)
	problemsTable.SetFixed(1, 0)

	problemSummary := tview.NewTextView().SetDynamicColors(true)
	problemSummary.SetBackgroundColor(tcell.ColorBlack)
	problemSummary.SetTextColor(tcell.ColorWhite)
	problemSummary.SetWrap(false)

	problemBar := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	problemBar.SetBackgroundColor(tcell.ColorBlack)
	problemBar.SetTextColor(tcell.ColorWhite)

	problemDrawer := tview.NewFlex().SetDirection(tview.FlexRow)
	problemDrawer.SetBackgroundColor(tcell.ColorBlack)
	problemDrawer.
		AddItem(problemsTable, 0, 1, false).
		AddItem(problemSummary, 1, 0, false)

	vm := &viewModel{
		app:              app,
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
	}

	vm.table.SetSelectedFunc(func(row, column int) {
		vm.updateDetail(row)
	})
	vm.table.SetSelectionChangedFunc(func(row, column int) {
		vm.updateDetail(row)
	})

	// k9s-style focus handling to highlight active component
	vm.table.SetFocusFunc(func() {
		vm.table.SetBorderColor(tcell.ColorSkyblue)
		vm.detail.SetBorderColor(tcell.ColorSlateGray)
		vm.logView.SetBorderColor(tcell.ColorSlateGray)
	})

	vm.detail.SetFocusFunc(func() {
		vm.table.SetBorderColor(tcell.ColorSlateGray)
		vm.detail.SetBorderColor(tcell.ColorSkyblue)
		vm.logView.SetBorderColor(tcell.ColorSlateGray)
	})

	vm.logView.SetFocusFunc(func() {
		vm.table.SetBorderColor(tcell.ColorSlateGray)
		vm.detail.SetBorderColor(tcell.ColorSlateGray)
		vm.logView.SetBorderColor(tcell.ColorSkyblue)
	})

	vm.root = tview.NewPages()
	vm.root.SetBackgroundColor(tcell.ColorBlack)
	vm.root.AddPage("main", vm.buildMainLayout(), true, true)

	app.SetRoot(vm.root, true)
	app.SetFocus(vm.table)
	vm.setCommandBar("queue")

	return vm
}

func (vm *viewModel) buildMainLayout() tview.Primitive {
	// Header: dense two-line bar (stats + commands) with compact logo
	headerTop := tview.NewFlex().SetDirection(tview.FlexColumn)
	headerTop.SetBackgroundColor(tcell.ColorBlack)
	headerTop.
		AddItem(vm.logoView, 8, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(vm.statusView, 0, 1, false)

	vm.header = tview.NewFlex().SetDirection(tview.FlexRow)
	vm.header.SetBackgroundColor(tcell.ColorBlack)
	vm.header.
		AddItem(headerTop, 0, 1, false).
		AddItem(vm.cmdBar, 1, 0, false)

	// Create log view container with search status
	logContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	logContainer.SetBackgroundColor(tcell.ColorBlack)
	logContainer.
		AddItem(vm.logView, 0, 1, true).      // Main log content
		AddItem(vm.searchStatus, 1, 0, false) // Search status bar at bottom

	// Create main content pages for different views
	// Dual-pane queue view: table + detail side-by-side
	queuePane := tview.NewFlex().SetDirection(tview.FlexColumn)
	queuePane.SetBackgroundColor(tcell.ColorBlack)
	queuePane.
		AddItem(vm.table, 0, 40, true).
		AddItem(vm.detail, 0, 60, false)

	vm.mainContent = tview.NewPages()
	vm.mainContent.SetBackgroundColor(tcell.ColorBlack)
	vm.mainContent.AddPage("queue", queuePane, true, true)
	vm.mainContent.AddPage("logs", logContainer, true, false)

	// Main layout: header + content + optional problems drawer
	vm.mainLayout = tview.NewFlex().SetDirection(tview.FlexRow)
	vm.mainLayout.SetBackgroundColor(tcell.ColorBlack)
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
		vm.detail.SetText("[cadetblue]Queue is empty.[-]\nAdd discs to Spindle or check logs at:\n[dodgerblue]" + tview.Escape(vm.options.Config.DaemonLogPath()) + "[-]")
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
			{"<Tab>", "Switch"},
			{"</>", "Search"},
			{"<n>/<N>", "Next/Prev"},
			{"<p>", "Problems"},
			{"<q>", "Queue"},
			{"<h>", "Help"},
			{"<e>", "Exit"},
		}
	case "detail":
		commands = []cmd{
			{"<q>", "Queue"},
			{"<l>", "Logs"},
			{"<i>", "Item Log"},
			{"<Tab>", "Switch"},
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
			{"<r>", "Encoding"},
			{"<i>", "Item Log"},
			{"<f>", "Filter: " + filterLabel},
			{"<p>", "Problems"},
			{"<Tab>", "Switch"},
			{"<h>", "Help"},
			{"<e>", "Exit"},
		}
	}

	segments := make([]string, 0, len(commands))
	for _, cmd := range commands {
		segments = append(segments, fmt.Sprintf("[#38bdf8]%s[-] [#64748b]%s[-]", cmd.key, cmd.desc)) // Sky-400, Slate-500
	}
	vm.cmdBar.SetText(strings.Join(segments, "  •  "))
}
