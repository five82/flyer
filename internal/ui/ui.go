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
		// Apply blue color to figlet output
		return applyBlueColor(string(output))
	}

	// Fallback: simple yellow FLYER text
	return "[yellow]FLYER[-]"
}

// applyBlueColor applies blue color to text using tview color tags
func applyBlueColor(text string) string {
	color := "[blue]"
	var result strings.Builder
	lines := strings.Split(text, "\n")

	for lineIdx, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		result.WriteString(color) // Start blue color for this line
		for _, r := range line {
			result.WriteRune(r)
		}
		result.WriteString("[-]") // End blue color for this line

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
	cmdView    *tview.TextView
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
	// Header components
	statusView := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	statusView.SetTextAlign(tview.AlignLeft)

	cmdView := tview.NewTextView().SetDynamicColors(true)
	cmdView.SetTextAlign(tview.AlignCenter)
	cmdView.SetText("[::b]Commands:[-] q:Queue  d:Detail  l:Logs  Tab:Switch  ?:Help  e:Exit")

	logoView := tview.NewTextView()
	logoView.SetTextAlign(tview.AlignRight)
	logoView.SetDynamicColors(true)
	logoView.SetRegions(true)
	logoView.SetText(createLogo())

	// Main content components
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Queue ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	detail := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	detail.SetBorder(true).SetTitle(" Details ")

	logView := tview.NewTextView().SetDynamicColors(true)
	logView.SetBorder(true).SetTitle(" Daemon Log ")
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

	vm.root = tview.NewPages()
	vm.root.AddPage("main", vm.buildMainLayout(), true, true)

	app.SetRoot(vm.root, true)
	app.SetFocus(vm.table)

	return vm
}

func (vm *viewModel) buildMainLayout() tview.Primitive {
	// Create header with status (left), commands (center), logo (right), padding
	// Using k9s-style fixed-width approach with proper right padding
	vm.header = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(vm.statusView, 50, 1, false).  // Fixed width for status
		AddItem(vm.cmdView, 0, 1, false).      // Takes remaining space
		AddItem(vm.logoView, 30, 1, false).    // Fixed width for logo
		AddItem(nil, 2, 1, false)              // Fixed padding space on right

	// Create main content pages for different views
	vm.mainContent = tview.NewPages()
	vm.mainContent.AddPage("queue", vm.table, true, true)
	vm.mainContent.AddPage("detail", vm.detail, true, false)
	vm.mainContent.AddPage("logs", vm.logView, true, false)

	// Main layout: header (25%) + main content (75%)
	main := tview.NewFlex().SetDirection(tview.FlexRow).
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
			vm.statusView.SetText(fmt.Sprintf("[red]spindle unavailable[-]\nRetrying (last attempt %s)", last))
			return
		}
		vm.statusView.SetText("[yellow]waiting for spindle status…[-]")
		return
	}
	stats := snapshot.Status.Workflow.QueueStats
	counts := []string{
		fmt.Sprintf("[blue]Pending[-]: %d", stats["pending"]),
		fmt.Sprintf("[cyan]Processing[-]: %d", stats["identifying"]+stats["ripping"]+stats["encoding"]+stats["organizing"]),
		fmt.Sprintf("[red]Failed[-]: %d", stats["failed"]),
		fmt.Sprintf("[yellow]Review[-]: %d", stats["review"]),
		fmt.Sprintf("[green]Completed[-]: %d", stats["completed"]),
	}
	summary := strings.Join(counts, "  ")
	statusText := fmt.Sprintf("[white]Daemon:[-] %s  [white]PID:[-] %d  [white]Updated:[-] %s\n%s",
		yesNo(snapshot.Status.Running), snapshot.Status.PID, snapshot.LastUpdated.Format("15:04:05"), summary)
	if snapshot.LastError != nil {
		statusText += fmt.Sprintf("  [red]Error:[-] %v", snapshot.LastError)
	}
	vm.statusView.SetText(statusText)
}


func (vm *viewModel) renderTable() {
	vm.table.Clear()
	headers := []string{"ID", "Title", "Status", "Lane", "Progress"}
	for col, label := range headers {
		vm.table.SetCell(0, col, tview.NewTableCell("[::b]"+label).SetSelectable(false))
	}

	rows := vm.items
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].ID > rows[j].ID
	})

	for row := 0; row < len(rows); row++ {
		item := rows[row]
		vm.table.SetCell(row+1, 0, tview.NewTableCell(fmt.Sprintf("%d", item.ID)))
		vm.table.SetCell(row+1, 1, tview.NewTableCell(composeTitle(item)))
		vm.table.SetCell(row+1, 2, tview.NewTableCell(strings.ToUpper(item.Status)))
		vm.table.SetCell(row+1, 3, tview.NewTableCell(determineLane(item.Status)))
		vm.table.SetCell(row+1, 4, tview.NewTableCell(formatProgress(item)))
	}
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
		vm.detail.SetText("Select an item to view details")
		return
	}
	item := vm.items[row-1]
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("[white]Title:[-] %s\n", composeTitle(item)))
	builder.WriteString(fmt.Sprintf("[white]Status:[-] %s\n", strings.ToUpper(item.Status)))
	builder.WriteString(fmt.Sprintf("[white]Lane:[-] %s\n", determineLane(item.Status)))
	if item.Progress.Stage != "" || item.Progress.Message != "" {
		builder.WriteString(fmt.Sprintf("[white]Stage:[-] %s (%s %.0f%%)\n", strings.TrimSpace(item.Progress.Stage), strings.TrimSpace(item.Progress.Message), item.Progress.Percent))
	}
	if strings.TrimSpace(item.ErrorMessage) != "" {
		builder.WriteString(fmt.Sprintf("[red]Error:[-] %s\n", item.ErrorMessage))
	}
	if item.NeedsReview {
		builder.WriteString("[yellow]Needs review[-]\n")
	}
	if item.ReviewReason != "" {
		builder.WriteString(fmt.Sprintf("[yellow]Reason:[-] %s\n", item.ReviewReason))
	}
	if item.RippedFile != "" {
		builder.WriteString(fmt.Sprintf("[white]Rip:[-] %s\n", item.RippedFile))
	}
	if item.EncodedFile != "" {
		builder.WriteString(fmt.Sprintf("[white]Encoded:[-] %s\n", item.EncodedFile))
	}
	if item.FinalFile != "" {
		builder.WriteString(fmt.Sprintf("[white]Final:[-] %s\n", item.FinalFile))
	}
	if item.BackgroundLogPath != "" {
		builder.WriteString(fmt.Sprintf("[white]Background log:[-] %s\n", item.BackgroundLogPath))
	}
	if ts := item.ParsedCreatedAt(); !ts.IsZero() {
		builder.WriteString(fmt.Sprintf("[white]Created:[-] %s\n", ts.Format(time.RFC3339)))
	}
	if ts := item.ParsedUpdatedAt(); !ts.IsZero() {
		builder.WriteString(fmt.Sprintf("[white]Updated:[-] %s\n", ts.Format(time.RFC3339)))
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
		vm.logView.SetTitle(" Item Log ")
	} else {
		vm.logMode = logSourceDaemon
		vm.logView.SetTitle(" Daemon Log ")
	}
	vm.lastLogPath = ""
	// Always show logs view when toggling log source
	vm.showLogsView()
}


func (vm *viewModel) showHelp() {
	text := "q Queue View  |  d Detail View  |  l Toggle Log Source  |  Tab Switch Views (Queue→Detail→Logs)  |  ? Help  |  e Exit  |  Ctrl+C Exit"
	modal := tview.NewModal().SetText(text).AddButtons([]string{"Close"})
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
