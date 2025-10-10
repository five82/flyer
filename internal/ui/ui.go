package ui

import (
	"context"
	"fmt"
	"os/exec"
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
		switch event.Key() {
		case tcell.KeyCtrlC:
			app.Stop()
			return nil
		case tcell.KeyTAB:
			model.toggleFocus()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
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

	root *tview.Pages

	items       []spindle.QueueItem
	logMode     logSource
	lastLogPath string
	lastLogSet  time.Time
	currentView string // "queue", "detail", "logs"
}

func newViewModel(app *tview.Application, opts Options) *viewModel {
	// Header components (k9s-style)
	statusView := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	statusView.SetTextAlign(tview.AlignLeft)
	statusView.SetBackgroundColor(tcell.ColorBlack)

	// k9s-style command menu using table layout for vertical columns
	cmdView := tview.NewTable()
	cmdView.SetBackgroundColor(tcell.ColorBlack)
	cmdView.SetBorders(false)

	// Define commands in k9s-style format
	commands := []struct{ key, desc string }{
		{"q", "Queue"},
		{"d", "Detail"},
		{"l", "Logs"},
		{"Tab", "Switch"},
		{"?", "Help"},
		{"e", "Exit"},
	}

	// Create single column layout with vertical alignment
	// Calculate max key length for proper alignment
	maxKeyLen := 0
	for _, cmd := range commands {
		if len(cmd.key) > maxKeyLen {
			maxKeyLen = len(cmd.key)
		}
	}

	for i, cmd := range commands {
		// Create properly aligned layout: <key>       description (k9s-style)
		paddedKey := fmt.Sprintf("<%s>", cmd.key)
		// Add spacing to align descriptions vertically (max key length + brackets + 5 spaces for k9s-style spacing)
		padding := strings.Repeat(" ", maxKeyLen+5-len(paddedKey))
		// k9s-style bold keys with proper coloring
		text := fmt.Sprintf("[::b][dodgerblue]%s[white]%s%s", paddedKey, padding, cmd.desc)
		cell := tview.NewTableCell(text)
		cell.SetBackgroundColor(tcell.ColorBlack)
		cell.SetSelectable(false)
		cell.SetAlign(tview.AlignLeft)
		cmdView.SetCell(i, 0, cell)  // All commands in column 0
	}

	logoView := tview.NewTextView()
	logoView.SetTextAlign(tview.AlignRight)
	logoView.SetDynamicColors(true)
	logoView.SetRegions(true)
	logoView.SetBackgroundColor(tcell.ColorBlack)
	logoView.SetText(createLogo())

	// Main content components (k9s-style)
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" [aqua]Queue[-] ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)
	table.SetBackgroundColor(tcell.ColorBlack)
	// k9s-style border color
	table.SetBorderColor(tcell.ColorDodgerBlue)

	detail := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	detail.SetBorder(true).SetTitle(" [aqua]Details[-] ")
	detail.SetBackgroundColor(tcell.ColorBlack)
	detail.SetBorderColor(tcell.ColorDodgerBlue)

	logView := tview.NewTextView().SetDynamicColors(true)
	logView.SetBorder(true).SetTitle(" [aqua]Daemon Log[-] ")
	logView.SetBackgroundColor(tcell.ColorBlack)
	logView.SetBorderColor(tcell.ColorDodgerBlue)
	logView.ScrollToEnd()

	vm := &viewModel{
		app:         app,
		options:     opts,
		statusView:  statusView,
		cmdView:     cmdView,
		logoView:    logoView,
		table:       table,
		detail:      detail,
		logView:     logView,
		currentView: "queue",
	}

	vm.table.SetSelectedFunc(func(row, column int) {
		vm.showDetailView()
	})

	// k9s-style focus handling to highlight active component
	vm.table.SetFocusFunc(func() {
		vm.table.SetBorderColor(tcell.ColorAqua)
		vm.detail.SetBorderColor(tcell.ColorDodgerBlue)
		vm.logView.SetBorderColor(tcell.ColorDodgerBlue)
	})

	vm.detail.SetFocusFunc(func() {
		vm.table.SetBorderColor(tcell.ColorDodgerBlue)
		vm.detail.SetBorderColor(tcell.ColorAqua)
		vm.logView.SetBorderColor(tcell.ColorDodgerBlue)
	})

	vm.logView.SetFocusFunc(func() {
		vm.table.SetBorderColor(tcell.ColorDodgerBlue)
		vm.detail.SetBorderColor(tcell.ColorDodgerBlue)
		vm.logView.SetBorderColor(tcell.ColorAqua)
	})

	vm.root = tview.NewPages()
	vm.root.SetBackgroundColor(tcell.ColorBlack)
	vm.root.AddPage("main", vm.buildMainLayout(), true, true)

	app.SetRoot(vm.root, true)
	app.SetFocus(vm.table)

	return vm
}

func (vm *viewModel) buildMainLayout() tview.Primitive {
	// Create padding elements for the header (1 character padding on each side)
	paddingLeft := tview.NewBox().SetBackgroundColor(tcell.ColorBlack)
	paddingRight := tview.NewBox().SetBackgroundColor(tcell.ColorBlack)

	// Create header with k9s-style proportions: padding (1), Status (30%), Commands (40%), Logo (30%), padding (1)
	vm.header = tview.NewFlex().SetDirection(tview.FlexColumn)
	vm.header.SetBackgroundColor(tcell.ColorBlack)
	vm.header.
		AddItem(paddingLeft, 1, 0, false).     // Left padding of 1 character
		AddItem(vm.statusView, 0, 30, false).  // Status ~30% width
		AddItem(vm.cmdView, 0, 40, false).     // Commands ~40% width
		AddItem(vm.logoView, 0, 30, false).    // Logo ~30% width
		AddItem(paddingRight, 1, 0, false)     // Right padding of 1 character

	// Create main content pages for different views
	vm.mainContent = tview.NewPages()
	vm.mainContent.SetBackgroundColor(tcell.ColorBlack)
	vm.mainContent.AddPage("queue", vm.table, true, true)
	vm.mainContent.AddPage("detail", vm.detail, true, false)
	vm.mainContent.AddPage("logs", vm.logView, true, false)

	// Main layout: header (25%) + main content (75%) - k9s proportions
	main := tview.NewFlex().SetDirection(tview.FlexRow)
	main.SetBackgroundColor(tcell.ColorBlack)
	main.
		AddItem(vm.header, 0, 1, false).   // Top area ~25%
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
		headerCell := tview.NewTableCell("[::b][black:white]"+label+"[-]")
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
	row, _ := vm.table.GetSelection()
	vm.updateDetail(row)
	vm.app.SetFocus(vm.detail)
}

func (vm *viewModel) showQueueView() {
	vm.currentView = "queue"
	vm.mainContent.SwitchToPage("queue")
	vm.app.SetFocus(vm.table)
}

func (vm *viewModel) showLogsView() {
	vm.currentView = "logs"
	vm.mainContent.SwitchToPage("logs")
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
	vm.logView.SetText(strings.Join(lines, "\n"))
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
		vm.showLogsView()
	case "logs":
		vm.showQueueView()
	}
}

func (vm *viewModel) toggleLogSource() {
	if vm.logMode == logSourceDaemon {
		vm.logMode = logSourceItem
		vm.logView.SetTitle(" [aqua]Item Log[-] ")
	} else {
		vm.logMode = logSourceDaemon
		vm.logView.SetTitle(" [aqua]Daemon Log[-] ")
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
		{"Tab", "Switch Views (Queue→Detail→Logs)"},
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

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
