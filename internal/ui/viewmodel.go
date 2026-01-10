package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/prefs"
	"github.com/five82/flyer/internal/spindle"
)

// searchState groups log search state fields.
type searchState struct {
	input        *tview.InputField
	hint         *tview.TextView
	status       *tview.TextView
	regex        *regexp.Regexp
	matches      []int
	currentMatch int
	lastPattern  string
	mode         bool
}

// queueSearchState groups queue filtering state fields.
type queueSearchState struct {
	input   *tview.InputField
	hint    *tview.TextView
	regex   *regexp.Regexp
	pattern string
	mode    bool
}

// logViewState groups log viewing state fields.
type logViewState struct {
	mode            logSource
	follow          bool
	lastPath        string
	lastKey         string
	lastFileKey     string
	lastSet         time.Time
	rawLines        []string
	cursor          map[string]uint64
	fileCursor      map[string]int64
	currentItemID   int64
	filterComponent string
	filterLane      string
	filterRequest   string
}

// problemsState groups problems view state fields.
type problemsState struct {
	logLines  []string
	logCursor map[string]uint64
	lastKey   string
	lastSet   time.Time
}

// detailViewState groups detail view state fields.
type detailViewState struct {
	lastID           int64
	episodeCollapsed map[int64]bool
	pathExpanded     map[int64]bool
	fullscreenMode   bool
}

type viewModel struct {
	// Core application state
	app     *tview.Application
	ctx     context.Context
	client  *spindle.Client
	options Options
	root    *tview.Pages
	theme   Theme

	// Header components
	header      *tview.Flex
	statusView  *tview.TextView
	cmdBar      *tview.TextView
	logoView    *tview.TextView
	lastRefresh time.Time

	// Main content views
	mainContent  *tview.Pages
	queuePane    *tview.Flex
	table        *tview.Table
	detail       *tview.TextView
	logView      *tview.TextView
	problemsView *tview.TextView
	mainLayout   *tview.Flex
	queueLayout  string // "wide" or "stacked"

	// Grouped state
	search      searchState
	queueSearch queueSearchState
	logs        logViewState
	problems    problemsState
	detailState detailViewState

	// Data and navigation state
	items        []spindle.QueueItem
	displayItems []spindle.QueueItem
	currentView  string // "queue", "detail", "logs", "problems"
	filterMode   queueFilter
}

// newThemedInputField creates a styled input field with consistent theming.
func (vm *viewModel) newThemedInputField(label string, width int) *tview.InputField {
	input := tview.NewInputField()
	input.SetLabel(label)
	input.SetFieldWidth(width)
	input.SetBackgroundColor(vm.theme.SurfaceColor())
	input.SetFieldBackgroundColor(vm.theme.SurfaceAltColor())
	input.SetFieldTextColor(hexToColor(vm.theme.Text.Primary))
	return input
}

// newSearchContainer creates a flex container for search/filter inputs.
func (vm *viewModel) newSearchContainer(hint *tview.TextView, input *tview.InputField) *tview.Flex {
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.SetBackgroundColor(vm.theme.SurfaceColor())
	container.AddItem(nil, 0, 1, false)
	container.AddItem(hint, 1, 0, false)
	container.AddItem(input, 1, 0, true)
	return container
}

func newViewModel(app *tview.Application, opts Options) *viewModel {
	// Load theme from saved preferences, falling back to default
	theme := GetTheme(opts.Prefs.Theme)

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

	problemsView := tview.NewTextView().SetDynamicColors(true)
	problemsView.SetBorder(true).SetTitle(" [::b]Problems[::-] ")
	problemsView.SetBackgroundColor(theme.SurfaceAltColor())
	problemsView.SetBorderColor(theme.TableBorderColor())
	problemsView.ScrollToEnd()

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	vm := &viewModel{
		app:          app,
		ctx:          ctx,
		client:       opts.Client,
		options:      opts,
		statusView:   statusView,
		cmdBar:       cmdBar,
		logoView:     logoView,
		table:        table,
		detail:       detail,
		logView:      logView,
		problemsView: problemsView,
		currentView:  "queue",
		theme:        theme,
		search: searchState{
			status: searchStatus,
		},
		logs: logViewState{
			follow:     true,
			cursor:     make(map[string]uint64),
			fileCursor: make(map[string]int64),
		},
		problems: problemsState{
			logCursor: make(map[string]uint64),
		},
		detailState: detailViewState{
			episodeCollapsed: map[int64]bool{},
			pathExpanded:     map[int64]bool{},
		},
	}

	vm.logView.SetInputCapture(vm.makeLogViewHandler())
	vm.detail.SetInputCapture(makeVimNavHandler(nil))
	vm.problemsView.SetInputCapture(makeVimNavHandler(nil))

	vm.table.SetSelectedFunc(func(row, column int) {
		vm.updateDetail(row)
	})
	vm.table.SetSelectionChangedFunc(func(row, column int) {
		vm.applySelectionStyling()
		vm.updateDetail(row)
	})

	vm.setupFocusHandlers()

	vm.root = tview.NewPages()
	vm.root.SetBackgroundColor(theme.BackgroundColor())
	vm.root.AddPage("main", vm.buildMainLayout(), true, true)

	app.SetRoot(vm.root, true)
	app.SetFocus(vm.table)
	vm.setCommandBar("queue")

	return vm
}

// cycleTheme switches to the next available theme and reapplies all colors.
func (vm *viewModel) cycleTheme() {
	nextName := NextTheme(vm.theme.Name)
	vm.theme = GetTheme(nextName)

	// Persist theme preference (ignore errors - graceful degradation)
	_ = prefs.Save(vm.options.PrefsPath, prefs.Prefs{Theme: nextName})

	// Update tview global styles
	tview.Styles.PrimitiveBackgroundColor = vm.theme.BackgroundColor()
	tview.Styles.ContrastBackgroundColor = vm.theme.SurfaceColor()
	tview.Styles.MoreContrastBackgroundColor = vm.theme.SurfaceAltColor()

	// Update root
	vm.root.SetBackgroundColor(vm.theme.BackgroundColor())

	// Update header components
	vm.statusView.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.statusView.SetTextColor(hexToColor(vm.theme.Text.Primary))
	vm.cmdBar.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.cmdBar.SetTextColor(hexToColor(vm.theme.Text.Secondary))
	vm.logoView.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.logoView.SetText(createLogo(vm.theme))

	// Update table
	vm.table.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.table.SetBorderColor(vm.theme.TableBorderColor())
	vm.table.SetSelectedStyle(tcell.StyleDefault.Background(vm.theme.TableSelectionBackground()).Foreground(vm.theme.TableSelectionTextColor()))

	// Update detail view
	vm.detail.SetBackgroundColor(vm.theme.SurfaceAltColor())
	vm.detail.SetBorderColor(vm.theme.TableBorderColor())

	// Update log view
	vm.logView.SetBackgroundColor(vm.theme.SurfaceAltColor())
	vm.logView.SetBorderColor(vm.theme.TableBorderColor())

	// Update problems view
	vm.problemsView.SetBackgroundColor(vm.theme.SurfaceAltColor())
	vm.problemsView.SetBorderColor(vm.theme.TableBorderColor())

	// Update search status
	if vm.search.status != nil {
		vm.search.status.SetBackgroundColor(vm.theme.SurfaceColor())
		vm.search.status.SetTextColor(hexToColor(vm.theme.Text.Secondary))
	}

	// Rebuild header backgrounds
	if vm.header != nil {
		vm.header.SetBackgroundColor(vm.theme.SurfaceColor())
	}

	// Reapply focus styles for current view
	vm.applyFocusStyles(vm.currentView)

	// Refresh content with new theme colors
	vm.renderTablePreservingSelection()
	if row, _ := vm.table.GetSelection(); row > 0 {
		vm.updateDetail(row)
	}

	// Update command bar to show theme change
	vm.setCommandBar(vm.currentView)
}

func (vm *viewModel) buildMainLayout() tview.Primitive {
	// Dynamic header layout based on terminal size
	headerTop := tview.NewFlex().SetDirection(tview.FlexColumn)
	headerTop.SetBackgroundColor(vm.theme.SurfaceColor())

	// Get terminal dimensions for responsive layout
	screenWidth := 120
	if vm.mainLayout != nil {
		_, _, w, _ := vm.mainLayout.GetRect()
		if w > 0 {
			screenWidth = w
		}
	}

	// Adjust header based on screen size
	if screenWidth >= 140 {
		// Large terminal - more spacious layout
		headerTop.
			AddItem(vm.logoView, 8, 0, false).
			AddItem(nil, 2, 0, false).
			AddItem(vm.statusView, 0, 1, false)
	} else if screenWidth >= 100 {
		// Medium terminal - standard layout
		headerTop.
			AddItem(vm.logoView, 6, 0, false).
			AddItem(nil, 1, 0, false).
			AddItem(vm.statusView, 0, 1, false)
	} else {
		// Small terminal - compact layout
		headerTop.
			AddItem(vm.logoView, 4, 0, false).
			AddItem(nil, 1, 0, false).
			AddItem(vm.statusView, 0, 1, false)
	}

	vm.header = tview.NewFlex().SetDirection(tview.FlexRow)
	vm.header.SetBackgroundColor(vm.theme.SurfaceColor())

	// Keep header compact for better monitoring
	vm.header.
		AddItem(headerTop, 0, 1, false).
		AddItem(vm.cmdBar, 1, 0, false)

	// Create log view container with search status
	logContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	logContainer.SetBackgroundColor(vm.theme.SurfaceColor())
	logContainer.
		AddItem(vm.logView, 0, 1, true).       // Main log content
		AddItem(vm.search.status, 1, 0, false) // Search status bar at bottom

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
	vm.mainContent.AddPage("problems", vm.problemsView, true, false)

	// Main layout: header + content
	vm.mainLayout = tview.NewFlex().SetDirection(tview.FlexRow)
	vm.mainLayout.SetBackgroundColor(vm.theme.BackgroundColor())
	vm.mainLayout.
		AddItem(vm.header, 2, 0, false). // keep header compact; status is one line + command bar
		AddItem(vm.mainContent, 0, 1, true)

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
	case "extra-wide":
		vm.queuePane.Clear()
		vm.queuePane.SetDirection(tview.FlexColumn)
		vm.queuePane.AddItem(vm.table, 0, 30, true) // Give more space to detail
		vm.queuePane.AddItem(vm.detail, 0, 70, false)
		vm.queueLayout = "extra-wide"
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
	height := 0
	if vm.mainLayout != nil {
		_, _, w, h := vm.mainLayout.GetRect()
		width = w
		height = h
	}
	if width <= 0 {
		_, _, w, h := vm.queuePane.GetRect()
		width = w
		height = h
	}

	// Enhanced layout logic for medium/large terminals
	target := "wide"
	if width > 0 {
		if width < LayoutCompactWidth {
			target = "stacked"
		} else if width >= LayoutExtraWideWidth {
			// Extra wide layout - give more space to detail
			vm.applyQueueLayout("extra-wide")
			return
		} else if height >= 40 {
			// Tall layout - can use stacked with better proportions
			target = "stacked"
		}
	}

	if target == vm.queueLayout {
		return
	}
	vm.applyQueueLayout(target)
}

func (vm *viewModel) ensureSelection() {
	itemCount := len(vm.displayItems)
	if itemCount == 0 {
		vm.table.Select(0, 0)
		vm.detail.SetText(fmt.Sprintf("[%s]Queue is empty.[-]\nAdd discs to Spindle or check logs at:\n[%s]%s[-]",
			vm.theme.Text.Muted,
			vm.theme.Text.Accent,
			tview.Escape(vm.options.Config.DaemonLogPath())))
		return
	}
	row, _ := vm.table.GetSelection()
	itemIdx := rowToItem(row)
	if itemIdx < 0 {
		vm.table.Select(itemToRow(0), 0)
	} else if itemIdx >= itemCount {
		vm.table.Select(itemToRow(itemCount-1), 0)
	}
	vm.applySelectionStyling()
}

func (vm *viewModel) setCommandBar(view string) {
	type cmd struct{ key, desc string }

	var commands []cmd
	switch view {
	case "logs":
		followLabel := "Pause"
		if !vm.logs.follow {
			followLabel = "Follow"
		}
		commands = []cmd{
			{"Space", followLabel},
			{"/", "Search"},
			{"n/N", "Next/Prev"},
			{"l", "Daemon"},
			{"i", "Item"},
			{"q", "Queue"},
			{"?", "More"},
		}
	case "problems":
		commands = []cmd{
			{"j/k", "Navigate"},
			{"l", "Daemon"},
			{"i", "Item"},
			{"q", "Queue"},
			{"Tab", "Focus"},
			{"?", "More"},
		}
	case "detail":
		fullscreenLabel := "Full"
		if vm.detailState.fullscreenMode {
			fullscreenLabel = "Split"
		}
		commands = []cmd{
			{"Enter", fullscreenLabel},
			{"t", "Episodes"},
			{"P", "Paths"},
			{"l", "Daemon"},
			{"i", "Item"},
			{"j/k", "Navigate"},
			{"q", "Queue"},
			{"?", "More"},
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
			{"/", "Search"},
			{"f", filterLabel},
			{"j/k", "Navigate"},
			{"d", "Details"},
			{"l", "Daemon"},
			{"i", "Item"},
			{"p", "Problems"},
			{"Tab", "Focus"},
			{"?", "More"},
		}
	}

	segments := make([]string, 0, len(commands))
	for _, cmd := range commands {
		segments = append(segments, fmt.Sprintf("[%s]%s[-]:[%s]%s[-]", vm.theme.Text.AccentSoft, cmd.key, vm.theme.Text.Secondary, cmd.desc))
	}
	if view != "logs" && vm.queueSearch.pattern != "" {
		pattern := truncate(vm.queueSearch.pattern, 18)
		segments = append(segments, fmt.Sprintf("[%s]/%s[-]", vm.theme.Text.Accent, tview.Escape(pattern)))
	}
	if view == "logs" && vm.search.lastPattern != "" {
		pattern := truncate(vm.search.lastPattern, 18)
		matchInfo := ""
		if len(vm.search.matches) > 0 {
			matchInfo = fmt.Sprintf(" [%s](%d)[-]", vm.theme.Text.Muted, len(vm.search.matches))
		}
		segments = append(segments, fmt.Sprintf("[%s]/%s[-]%s", vm.theme.Text.Accent, tview.Escape(pattern), matchInfo))
	}

	// Add theme indicator
	segments = append(segments, fmt.Sprintf("[%s]T[-]:[%s]%s[-]", vm.theme.Text.AccentSoft, vm.theme.Text.Muted, vm.theme.Name))

	vm.cmdBar.SetText(strings.Join(segments, "  "))
}
