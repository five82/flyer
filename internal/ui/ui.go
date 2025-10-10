package ui

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/config"
	"github.com/five82/flyer/internal/logtail"
	"github.com/five82/flyer/internal/spindle"
	"github.com/five82/flyer/internal/state"
)

// Options configure the UI runtime.
type Options struct {
	Store        *state.Store
	LogPath      string
	Config       config.Config
	RefreshEvery time.Duration
}

const (
	maxLogLines       = 400
	defaultUIInterval = time.Second
)

// createLogo generates the flyer logo using figlet or fallback
func createLogo() string {
	// Try to use figlet for ASCII art
	cmd := exec.Command("figlet", "-f", "slant", "flyer")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		// Apply orange color to figlet output (k9s-style logo color)
		return applyOrangeColor(string(output))
	}

	// Fallback: simple orange FLYER text (k9s-style)
	return "[orange]FLYER[-]"
}

// applyOrangeColor applies orange color to text using tview color tags (k9s-style)
func applyOrangeColor(text string) string {
	color := "[orange]"
	var result strings.Builder
	lines := strings.Split(text, "\n")

	for lineIdx, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		result.WriteString(color) // Start orange color for this line
		for _, r := range line {
			result.WriteRune(r)
		}
		result.WriteString("[-]") // End orange color for this line

		if lineIdx < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

type logSource int

const (
	logSourceDaemon logSource = iota
	logSourceItem
)

// Run wires up tview components and blocks until ctx is cancelled or the user quits.
func Run(ctx context.Context, opts Options) error {
	if opts.Store == nil {
		return fmt.Errorf("ui requires a data store")
	}

	app := tview.NewApplication()
	model := newViewModel(app, opts)

	refreshEvery := opts.RefreshEvery
	if refreshEvery <= 0 {
		refreshEvery = defaultUIInterval
	}
	if refreshEvery > time.Second {
		refreshEvery = time.Second
	}

	go func() {
		ticker := time.NewTicker(refreshEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				app.QueueUpdateDraw(func() { app.Stop() })
				return
			case <-ticker.C:
				snapshot := opts.Store.Snapshot()
				app.QueueUpdateDraw(func() {
					model.update(snapshot)
				})
			}
		}
	}()

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle search mode
		if model.searchMode {
			switch event.Key() {
			case tcell.KeyEnter:
				model.performSearch()
				return nil
			case tcell.KeyESC:
				model.cancelSearch()
				return nil
			case tcell.KeyCtrlC:
				app.Stop()
				return nil
			}
			return event
		}

		switch event.Key() {
		case tcell.KeyCtrlC:
			app.Stop()
			return nil
		case tcell.KeyESC:
			model.showQueueView()
			return nil
		case tcell.KeyTAB:
			model.toggleFocus()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case '/':
				if model.currentView == "logs" {
					model.startSearch()
				}
				return nil
			case 'n':
				model.nextSearchMatch()
				return nil
			case 'N':
				model.previousSearchMatch()
				return nil
			case 'q':
				model.showQueueView()
				return nil
			case 'e':
				app.Stop()
				return nil
			case 'd':
				model.showDetailView()
				return nil
			case 'l':
				model.toggleLogSource()
				return nil
			case 'i':
				model.showItemLogsView()
				return nil
			case '?':
				model.showHelp()
				return nil
			}
		}
		return event
	})

	go func() {
		<-ctx.Done()
		app.QueueUpdateDraw(func() { app.Stop() })
	}()

	return app.Run()
}

type viewModel struct {
	app     *tview.Application
	options Options

	// Header components (top 25%)
	header     *tview.Flex
	statusView *tview.TextView
	cmdView    *tview.Table
	logoView   *tview.TextView

	// Main content pane (bottom 75%)
	mainContent *tview.Pages
	table       *tview.Table
	detail      *tview.TextView
	logView     *tview.TextView

	// Search components
	searchInput        *tview.InputField
	searchStatus       *tview.TextView
	searchRegex        *regexp.Regexp
	searchMatches      []int
	currentSearchMatch int
	lastSearchPattern  string

	root *tview.Pages

	items       []spindle.QueueItem
	logMode     logSource
	lastLogPath string
	lastLogSet  time.Time
	currentView string // "queue", "detail", "logs"
	searchMode  bool   // true when in search input mode
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
	tview.Styles.PrimaryTextColor = tcell.ColorWhite

	// Header components (k9s-style)
	statusView := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	statusView.SetTextAlign(tview.AlignLeft)
	statusView.SetBackgroundColor(tcell.ColorBlack)

	// Commands section using Table (k9s Menu pattern)
	// k9s fills table with maxRows=6, flowing into multiple columns
	cmdView := tview.NewTable()
	cmdView.SetBackgroundColor(tcell.ColorBlack)

	commands := []struct{ key, desc string }{
		{"<q>", "Queue"},
		{"<d>", "Detail"},
		{"<l>", "Logs"},
		{"<i>", "Item Logs"},
		{"<Tab>", "Switch"},
		{"<?>", "Help"},
		{"<e>", "Exit"},
	}

	// k9s pattern: fill rows first, then columns (maxRows=6)
	// Calculate max key width per column for padding (k9s does this!)
	const maxRows = 6
	colCount := (len(commands) / maxRows) + 1

	// Find max key length per column
	maxKeyWidth := make([]int, colCount)
	for i, cmd := range commands {
		col := i / maxRows
		if len(cmd.key) > maxKeyWidth[col] {
			maxKeyWidth[col] = len(cmd.key)
		}
	}

	for i, cmd := range commands {
		row := i % maxRows
		col := i / maxRows

		// k9s format with padding: " <key>  description " (line 132 in menu.go)
		// Use %-Ns format to left-align and pad keys to same width
		paddedKey := fmt.Sprintf("%-*s", maxKeyWidth[col], cmd.key)
		cell := tview.NewTableCell(fmt.Sprintf(" [::b][dodgerblue]%s[white]  %s ", paddedKey, cmd.desc))
		cell.SetBackgroundColor(tcell.ColorBlack)
		cell.SetExpansion(1) // Make cells expand to fill available space

		cmdView.SetCell(row, col, cell)
	}

	// Fill empty cells so all columns render
	for row := 0; row < maxRows; row++ {
		for col := 0; col < colCount; col++ {
			if cmdView.GetCell(row, col) == nil {
				empty := tview.NewTableCell("")
				empty.SetBackgroundColor(tcell.ColorBlack)
				empty.SetExpansion(1) // Make empty cells expand too
				cmdView.SetCell(row, col, empty)
			}
		}
	}

	logoView := tview.NewTextView()
	logoView.SetTextAlign(tview.AlignRight)
	logoView.SetDynamicColors(true)
	logoView.SetRegions(true)
	logoView.SetBackgroundColor(tcell.ColorBlack)
	logoView.SetText(createLogo())

	// Main content components (k9s-style)
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" [lightskyblue]Queue[-] ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)
	table.SetBackgroundColor(tcell.ColorBlack)
	// k9s-style border color
	table.SetBorderColor(tcell.ColorLightSkyBlue)

	detail := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	detail.SetBorder(true).SetTitle(" [lightskyblue]Details[-] ")
	detail.SetBackgroundColor(tcell.ColorBlack)
	detail.SetBorderColor(tcell.ColorLightSkyBlue)

	logView := tview.NewTextView().SetDynamicColors(true)
	logView.SetBorder(true).SetTitle(" [lightskyblue]Daemon Log[-] ")
	logView.SetBackgroundColor(tcell.ColorBlack)
	logView.SetBorderColor(tcell.ColorLightSkyBlue)
	logView.ScrollToEnd()

	// Search status bar (vim-style at bottom)
	searchStatus := tview.NewTextView().SetDynamicColors(true)
	searchStatus.SetBackgroundColor(tcell.ColorBlack)
	searchStatus.SetTextColor(tcell.ColorWhite)

	vm := &viewModel{
		app:          app,
		options:      opts,
		statusView:   statusView,
		cmdView:      cmdView,
		logoView:     logoView,
		table:        table,
		detail:       detail,
		logView:      logView,
		searchStatus: searchStatus,
		currentView:  "queue",
	}

	vm.table.SetSelectedFunc(func(row, column int) {
		vm.showDetailView()
	})

	// k9s-style focus handling to highlight active component
	vm.table.SetFocusFunc(func() {
		vm.table.SetBorderColor(tcell.ColorLightSkyBlue)
		vm.detail.SetBorderColor(tcell.ColorLightSkyBlue)
		vm.logView.SetBorderColor(tcell.ColorLightSkyBlue)
	})

	vm.detail.SetFocusFunc(func() {
		vm.table.SetBorderColor(tcell.ColorLightSkyBlue)
		vm.detail.SetBorderColor(tcell.ColorLightSkyBlue)
		vm.logView.SetBorderColor(tcell.ColorLightSkyBlue)
	})

	vm.logView.SetFocusFunc(func() {
		vm.table.SetBorderColor(tcell.ColorLightSkyBlue)
		vm.detail.SetBorderColor(tcell.ColorLightSkyBlue)
		vm.logView.SetBorderColor(tcell.ColorLightSkyBlue)
	})

	vm.root = tview.NewPages()
	vm.root.SetBackgroundColor(tcell.ColorBlack)
	vm.root.AddPage("main", vm.buildMainLayout(), true, true)

	app.SetRoot(vm.root, true)
	app.SetFocus(vm.table)

	return vm
}

func (vm *viewModel) buildMainLayout() tview.Primitive {
	// k9s header layout: FIXED width left | FLEX middle | FIXED width right
	// ClusterInfo: 50 chars | Menu: remaining space | Logo: 26 chars
	const (
		statusWidth = 40 // Fixed width for status section (k9s uses 50 for cluster info)
		logoWidth   = 30 // Fixed width for logo section (k9s uses 26)
	)

	// k9s-style header: FIXED | FLEX | FIXED (exact k9s pattern)
	// Add single space padding on left and right edges
	vm.header = tview.NewFlex().SetDirection(tview.FlexColumn)
	vm.header.SetBackgroundColor(tcell.ColorBlack)
	vm.header.
		AddItem(nil, 1, 1, false).                     // Single space padding left
		AddItem(vm.statusView, statusWidth, 1, false). // FIXED 40 chars
		AddItem(vm.cmdView, 0, 1, false).              // FLEX - direct table (k9s does this!)
		AddItem(vm.logoView, logoWidth, 1, false).     // FIXED 30 chars
		AddItem(nil, 1, 1, false)                      // Single space padding right

	// Create log view container with search status
	logContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	logContainer.SetBackgroundColor(tcell.ColorBlack)
	logContainer.
		AddItem(vm.logView, 0, 1, true).      // Main log content
		AddItem(vm.searchStatus, 1, 0, false) // Search status bar at bottom

	// Create main content pages for different views
	vm.mainContent = tview.NewPages()
	vm.mainContent.SetBackgroundColor(tcell.ColorBlack)
	vm.mainContent.AddPage("queue", vm.table, true, true)
	vm.mainContent.AddPage("detail", vm.detail, true, false)
	vm.mainContent.AddPage("logs", logContainer, true, false)

	// Main layout: header (25%) + main content (75%) - k9s proportions
	main := tview.NewFlex().SetDirection(tview.FlexRow)
	main.SetBackgroundColor(tcell.ColorBlack)
	main.
		AddItem(vm.header, 0, 1, false).    // Top area ~25%
		AddItem(vm.mainContent, 0, 3, true) // Main pane ~75%

	return main
}

func (vm *viewModel) update(snapshot state.Snapshot) {
	vm.renderStatus(snapshot)
	vm.items = snapshot.Queue
	vm.renderTable()
	vm.ensureSelection()

	if vm.currentView == "detail" {
		row, _ := vm.table.GetSelection()
		vm.updateDetail(row)
	}
	if vm.currentView == "logs" {
		vm.refreshLogs(false)
	}
}

func (vm *viewModel) renderStatus(snapshot state.Snapshot) {
	if !snapshot.HasStatus {
		if snapshot.LastError != nil {
			last := "soon"
			if !snapshot.LastUpdated.IsZero() {
				last = snapshot.LastUpdated.Format("15:04:05")
			}
			vm.statusView.SetText(fmt.Sprintf("[orangered]spindle unavailable[-]\nRetrying (last attempt [cadetblue]%s[-])", last))
			return
		}
		vm.statusView.SetText("[darkorange]waiting for spindle status…[-]")
		return
	}
	stats := snapshot.Status.Workflow.QueueStats
	counts := []string{
		fmt.Sprintf("[lightskyblue]Pending[-]: [dodgerblue]%d[-]", stats["pending"]),
		fmt.Sprintf("[greenyellow]Processing[-]: [dodgerblue]%d[-]", stats["identifying"]+stats["ripping"]+stats["encoding"]+stats["organizing"]),
		fmt.Sprintf("[orangered]Failed[-]: [dodgerblue]%d[-]", stats["failed"]),
		fmt.Sprintf("[darkorange]Review[-]: [dodgerblue]%d[-]", stats["review"]),
		fmt.Sprintf("[greenyellow]Completed[-]: [dodgerblue]%d[-]", stats["completed"]),
	}

	// k9s-style daemon status with colors
	daemonStatus := "[red]no[-]"
	if snapshot.Status.Running {
		daemonStatus = "[greenyellow]yes[-]"
	}

	statusText := fmt.Sprintf("[fuchsia]Daemon:[-] %s\n[fuchsia]PID:[-] [dodgerblue]%d[-]\n[fuchsia]Updated:[-] [cadetblue]%s[-]\n%s",
		daemonStatus, snapshot.Status.PID, snapshot.LastUpdated.Format("15:04:05"), strings.Join(counts, "\n"))
	if snapshot.LastError != nil {
		statusText += fmt.Sprintf("\n[orangered]Error:[-] [red]%v[-]", snapshot.LastError)
	}
	vm.statusView.SetText(statusText)
}

func (vm *viewModel) renderTable() {
	vm.table.Clear()
	headers := []string{"ID", "Title", "Status", "Lane", "Progress"}
	for col, label := range headers {
		// k9s-style table headers with white background
		headerCell := tview.NewTableCell("[::b][black:white]" + label + "[-]")
		headerCell.SetSelectable(false)
		vm.table.SetCell(0, col, headerCell)
	}

	rows := vm.items
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].ID > rows[j].ID
	})

	for row := 0; row < len(rows); row++ {
		item := rows[row]
		// k9s-style table data with dodgerblue color
		vm.table.SetCell(row+1, 0, tview.NewTableCell(fmt.Sprintf("[dodgerblue]%d[-]", item.ID)))
		vm.table.SetCell(row+1, 1, tview.NewTableCell("[dodgerblue]"+composeTitle(item)+"[-]"))
		vm.table.SetCell(row+1, 2, tview.NewTableCell("[dodgerblue]"+strings.ToUpper(item.Status)+"[-]"))
		vm.table.SetCell(row+1, 3, tview.NewTableCell("[dodgerblue]"+determineLane(item.Status)+"[-]"))
		vm.table.SetCell(row+1, 4, tview.NewTableCell("[dodgerblue]"+formatProgress(item)+"[-]"))
	}

	// Set k9s-style selection colors
	vm.table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorAqua).Foreground(tcell.ColorBlack))
}

func composeTitle(item spindle.QueueItem) string {
	title := strings.TrimSpace(item.DiscTitle)
	if title != "" {
		return title
	}
	return fallbackTitle(item.SourcePath)
}

func fallbackTitle(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "Unknown"
	}
	parts := strings.Split(trimmed, "/")
	name := parts[len(parts)-1]
	if name == "" && len(parts) > 1 {
		return parts[len(parts)-2]
	}
	return name
}

func formatProgress(item spindle.QueueItem) string {
	stage := strings.TrimSpace(item.Progress.Stage)
	if stage == "" {
		stage = titleCase(item.Status)
	}
	percent := item.Progress.Percent
	if percent <= 0 {
		return stage
	}
	return fmt.Sprintf("%s %.0f%%", stage, percent)
}

func determineLane(status string) string {
	switch status {
	case "pending", "identifying", "identified", "ripping":
		return "foreground"
	case "ripped", "encoding", "encoded", "organizing", "completed":
		return "background"
	case "failed", "review":
		return "attention"
	default:
		return "-"
	}
}

func titleCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}

func (vm *viewModel) ensureSelection() {
	rows := len(vm.items)
	if rows == 0 {
		vm.table.Select(0, 0)
		vm.detail.SetText("Queue is empty")
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

func (vm *viewModel) showDetailView() {
	vm.currentView = "detail"
	vm.mainContent.SwitchToPage("detail")
	vm.clearSearch()
	row, _ := vm.table.GetSelection()
	vm.updateDetail(row)
	vm.app.SetFocus(vm.detail)
}

func (vm *viewModel) showQueueView() {
	vm.currentView = "queue"
	vm.mainContent.SwitchToPage("queue")
	vm.clearSearch()
	vm.app.SetFocus(vm.table)
}

func (vm *viewModel) showLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
}

func (vm *viewModel) showItemLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	// Force item log mode
	vm.logMode = logSourceItem
	vm.logView.SetTitle(" [lightskyblue]Item Log[-] ")
	vm.lastLogPath = ""
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
}

func (vm *viewModel) showDaemonLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
	// Force daemon log mode
	vm.logMode = logSourceDaemon
	vm.logView.SetTitle(" [lightskyblue]Daemon Log[-] ")
	vm.lastLogPath = ""
	vm.refreshLogs(true)
	vm.app.SetFocus(vm.logView)
}

func (vm *viewModel) updateDetail(row int) {
	if row <= 0 || row-1 >= len(vm.items) {
		vm.detail.SetText("[cadetblue]Select an item to view details[-]")
		return
	}
	item := vm.items[row-1]
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("[fuchsia]Title:[-] [dodgerblue]%s[-]\n", composeTitle(item)))
	builder.WriteString(fmt.Sprintf("[fuchsia]Status:[-] [dodgerblue]%s[-]\n", strings.ToUpper(item.Status)))
	builder.WriteString(fmt.Sprintf("[fuchsia]Lane:[-] [dodgerblue]%s[-]\n", determineLane(item.Status)))
	if item.Progress.Stage != "" || item.Progress.Message != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Stage:[-] [dodgerblue]%s[-] ([cadetblue]%s[-] [dodgerblue]%.0f%%[-])\n", strings.TrimSpace(item.Progress.Stage), strings.TrimSpace(item.Progress.Message), item.Progress.Percent))
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		builder.WriteString(fmt.Sprintf("[orangered]Error:[-] [red]%s[-]\n", item.ErrorMessage))
	}
	if item.NeedsReview {
		builder.WriteString("[darkorange]Needs review[-]\n")
	}
	if item.ReviewReason != "" {
		builder.WriteString(fmt.Sprintf("[darkorange]Reason:[-] [dodgerblue]%s[-]\n", item.ReviewReason))
	}
	if item.RippedFile != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Rip:[-] [cadetblue]%s[-]\n", item.RippedFile))
	}
	if item.EncodedFile != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Encoded:[-] [cadetblue]%s[-]\n", item.EncodedFile))
	}
	if item.FinalFile != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Final:[-] [cadetblue]%s[-]\n", item.FinalFile))
	}
	if item.BackgroundLogPath != "" {
		builder.WriteString(fmt.Sprintf("[fuchsia]Background log:[-] [cadetblue]%s[-]\n", item.BackgroundLogPath))
	}
	if ts := item.ParsedCreatedAt(); !ts.IsZero() {
		builder.WriteString(fmt.Sprintf("[fuchsia]Created:[-] [cadetblue]%s[-]\n", ts.Format(time.RFC3339)))
	}
	if ts := item.ParsedUpdatedAt(); !ts.IsZero() {
		builder.WriteString(fmt.Sprintf("[fuchsia]Updated:[-] [cadetblue]%s[-]\n", ts.Format(time.RFC3339)))
	}
	vm.detail.SetText(builder.String())
}

func (vm *viewModel) refreshLogs(force bool) {
	path := vm.options.LogPath
	if vm.logMode == logSourceItem {
		item := vm.selectedItem()
		if item == nil || strings.TrimSpace(item.BackgroundLogPath) == "" {
			vm.logView.SetText("No background log for this item")
			vm.lastLogPath = ""
			return
		}
		path = item.BackgroundLogPath
	}
	if path == "" {
		vm.logView.SetText("Log path not configured")
		return
	}
	if !force && path == vm.lastLogPath && time.Since(vm.lastLogSet) < 500*time.Millisecond {
		return
	}
	lines, err := logtail.Read(path, maxLogLines)
	if err != nil {
		vm.logView.SetText(fmt.Sprintf("Error reading log: %v", err))
		vm.lastLogPath = path
		vm.lastLogSet = time.Now()
		return
	}
	colorizedLines := logtail.ColorizeLines(lines)
	vm.logView.SetText(strings.Join(colorizedLines, "\n"))
	vm.lastLogPath = path
	vm.lastLogSet = time.Now()
}

func (vm *viewModel) selectedItem() *spindle.QueueItem {
	row, _ := vm.table.GetSelection()
	if row <= 0 || row-1 >= len(vm.items) {
		return nil
	}
	item := vm.items[row-1]
	return &item
}

func (vm *viewModel) toggleFocus() {
	switch vm.currentView {
	case "queue":
		vm.showDetailView()
	case "detail":
		vm.showDaemonLogsView()
	case "logs":
		if vm.logMode == logSourceDaemon {
			vm.showItemLogsView()
		} else {
			vm.showQueueView()
		}
	}
}

func (vm *viewModel) toggleLogSource() {
	if vm.logMode == logSourceDaemon {
		vm.logMode = logSourceItem
		vm.logView.SetTitle(" [lightskyblue]Item Log[-] ")
	} else {
		vm.logMode = logSourceDaemon
		vm.logView.SetTitle(" [lightskyblue]Daemon Log[-] ")
	}
	vm.lastLogPath = ""
	// Always show logs view when toggling log source
	vm.showLogsView()
}

func (vm *viewModel) showHelp() {
	// k9s-style help text with bracketed keys in column layout
	helpCommands := []struct{ key, desc string }{
		{"q", "Queue View"},
		{"d", "Detail View"},
		{"l", "Toggle Log Source"},
		{"i", "Item Logs (Highlighted)"},
		{"/", "Start New Search"},
		{"n", "Next Search Match"},
		{"N", "Previous Search Match"},
		{"Tab", "Cycle Views (Queue→Detail→Daemon→Item)"},
		{"ESC", "Return to Queue View"},
		{"?", "Help"},
		{"e", "Exit"},
		{"Ctrl+C", "Exit"},
	}

	// Create formatted help text
	var helpLines []string
	maxRows := 4
	for i, cmd := range helpCommands {
		row := i % maxRows
		col := i / maxRows

		text := fmt.Sprintf("[dodgerblue]<%s>[white] %s", cmd.key, cmd.desc)
		for len(helpLines) <= row {
			helpLines = append(helpLines, "")
		}
		if col > 0 {
			helpLines[row] += "  |  " + text
		} else {
			helpLines[row] = text
		}
	}

	text := strings.Join(helpLines, "\n")
	modal := tview.NewModal().SetText(text).AddButtons([]string{"Close"})
	// k9s-style modal styling
	modal.SetBorderColor(tcell.ColorDodgerBlue)
	modal.SetBackgroundColor(tcell.ColorBlack)
	modal.SetTextColor(tcell.ColorDodgerBlue)
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		vm.root.RemovePage("modal")
		vm.returnToCurrentView()
	})
	vm.root.RemovePage("modal")
	vm.root.AddPage("modal", center(75, 7, modal), true, true)
}

func (vm *viewModel) returnToCurrentView() {
	switch vm.currentView {
	case "queue":
		vm.app.SetFocus(vm.table)
	case "detail":
		vm.app.SetFocus(vm.detail)
	case "logs":
		vm.app.SetFocus(vm.logView)
	}
}

func center(width, height int, primitive tview.Primitive) tview.Primitive {
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(primitive, width, 0, true).
			AddItem(nil, 0, 1, false), height, 0, true).
		AddItem(nil, 0, 1, false)
}

func (vm *viewModel) startSearch() {
	if vm.currentView != "logs" {
		return
	}

	vm.searchMode = true
	vm.searchInput = tview.NewInputField()
	vm.searchInput.SetLabel("/")
	vm.searchInput.SetFieldWidth(40)
	vm.searchInput.SetBackgroundColor(tcell.ColorBlack)
	vm.searchInput.SetFieldTextColor(tcell.ColorWhite)

	// Create a simple container for the search input
	searchContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	searchContainer.SetBackgroundColor(tcell.ColorBlack)
	searchContainer.AddItem(nil, 0, 1, false) // Push to bottom
	searchContainer.AddItem(vm.searchInput, 1, 0, true)

	vm.searchInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			vm.performSearch()
		case tcell.KeyESC:
			vm.cancelSearch()
		}
	})

	vm.root.AddPage("search", searchContainer, true, true)
	vm.app.SetFocus(vm.searchInput)
}

func (vm *viewModel) performSearch() {
	if vm.searchInput == nil {
		return
	}
	searchText := strings.TrimSpace(vm.searchInput.GetText())
	if searchText == "" {
		vm.cancelSearch()
		return
	}

	// Compile regex for case-insensitive search
	regex, err := regexp.Compile("(?i)" + searchText)
	if err != nil {
		vm.cancelSearch()
		return
	}

	vm.searchRegex = regex
	vm.lastSearchPattern = searchText
	vm.root.RemovePage("search")
	vm.searchMode = false

	// Find matches in current log content
	vm.findSearchMatches()
	if len(vm.searchMatches) > 0 {
		vm.currentSearchMatch = 0
		vm.highlightSearchMatch()
		vm.updateSearchStatus()
	} else {
		vm.searchStatus.SetText(fmt.Sprintf("[red]Pattern not found: %s[-]", searchText))
	}
}

func (vm *viewModel) cancelSearch() {
	vm.root.RemovePage("search")
	vm.searchMode = false
	vm.returnToCurrentView()
}

func (vm *viewModel) clearSearch() {
	vm.searchRegex = nil
	vm.searchMatches = []int{}
	vm.currentSearchMatch = 0
	vm.lastSearchPattern = ""
	vm.searchStatus.SetText("")
}

func (vm *viewModel) updateSearchStatus() {
	if vm.searchRegex == nil || len(vm.searchMatches) == 0 {
		vm.searchStatus.SetText("")
		return
	}

	matchNum := vm.currentSearchMatch + 1
	totalMatches := len(vm.searchMatches)
	vm.searchStatus.SetText(fmt.Sprintf("[dodgerblue]/%s[-] - [yellow]%d/%d[-] - Press [dodgerblue]n[-] for next, [dodgerblue]N[-] for previous",
		vm.lastSearchPattern, matchNum, totalMatches))
}

func (vm *viewModel) findSearchMatches() {
	if vm.searchRegex == nil {
		return
	}

	logText := vm.logView.GetText(false)
	lines := strings.Split(logText, "\n")

	vm.searchMatches = []int{}
	for i, line := range lines {
		if vm.searchRegex.MatchString(line) {
			vm.searchMatches = append(vm.searchMatches, i)
		}
	}
}

func (vm *viewModel) nextSearchMatch() {
	if len(vm.searchMatches) == 0 {
		return
	}

	vm.currentSearchMatch = (vm.currentSearchMatch + 1) % len(vm.searchMatches)
	vm.highlightSearchMatch()
	vm.updateSearchStatus()
}

func (vm *viewModel) previousSearchMatch() {
	if len(vm.searchMatches) == 0 {
		return
	}

	vm.currentSearchMatch = (vm.currentSearchMatch - 1 + len(vm.searchMatches)) % len(vm.searchMatches)
	vm.highlightSearchMatch()
	vm.updateSearchStatus()
}

func (vm *viewModel) highlightSearchMatch() {
	if len(vm.searchMatches) == 0 || vm.currentSearchMatch >= len(vm.searchMatches) {
		return
	}

	targetLine := vm.searchMatches[vm.currentSearchMatch]

	// Get original log content (without highlighting)
	logText := vm.logView.GetText(false)
	lines := strings.Split(logText, "\n")

	// Highlight all matches, but emphasize the current one
	for i, line := range lines {
		if vm.searchRegex.MatchString(line) {
			if i == targetLine {
				// Current match: yellow background with bold text
				lines[i] = vm.searchRegex.ReplaceAllString(line, "[::b][black:yellow]${0}[-]")
			} else {
				// Other matches: just highlight in red
				lines[i] = vm.searchRegex.ReplaceAllString(line, "[red]${0}[-]")
			}
		}
	}

	// Update the log view with highlighted content
	vm.logView.SetText(strings.Join(lines, "\n"))

	// Scroll to the matched line
	vm.logView.ScrollTo(targetLine, 0)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
