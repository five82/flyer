package ui

import (
	"context"
	"fmt"
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
				app.Stop()
				return nil
			case 'l':
				model.toggleLogSource()
				return nil
			case '/':
				model.promptFilter()
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

	statusBar *tview.TextView
	filterBar *tview.TextView
	table     *tview.Table
	detail    *tview.TextView
	logView   *tview.TextView
	root      *tview.Pages

	filteredItems []spindle.QueueItem
	filterText    string
	logMode       logSource
	lastLogPath   string
	lastLogSet    time.Time
	focusOnTable  bool
}

func newViewModel(app *tview.Application, opts Options) *viewModel {
	statusBar := tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true)
	statusBar.SetTextAlign(tview.AlignLeft)

	vm := &viewModel{
		app:          app,
		options:      opts,
		statusBar:    statusBar,
		filterBar:    tview.NewTextView().SetDynamicColors(true),
		table:        tview.NewTable(),
		detail:       tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		logView:      tview.NewTextView().SetDynamicColors(true),
		focusOnTable: true,
	}

	vm.table.SetBorder(true).SetTitle(" Queue ")
	vm.table.SetSelectable(true, false)
	vm.table.SetFixed(1, 0)
	vm.detail.SetBorder(true).SetTitle(" Details ")
	vm.logView.SetBorder(true).SetTitle(" Daemon Log ")
	vm.logView.ScrollToEnd()

	vm.table.SetSelectedFunc(func(row, column int) {
		vm.updateDetail(row)
		vm.refreshLogs(true)
	})

	vm.root = tview.NewPages()
	vm.root.AddPage("main", vm.buildMainLayout(), true, true)

	app.SetRoot(vm.root, true)
	app.SetFocus(vm.table)

	return vm
}

func (vm *viewModel) buildMainLayout() tview.Primitive {
	headers := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(vm.statusBar, 2, 0, false).
		AddItem(vm.filterBar, 1, 0, false)

	body := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(vm.table, 0, 2, true).
		AddItem(vm.detail, 0, 3, false)

	main := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(headers, 2, 0, false).
		AddItem(body, 0, 5, true).
		AddItem(vm.logView, 0, 3, false)

	return main
}

func (vm *viewModel) update(snapshot state.Snapshot) {
	vm.renderStatus(snapshot)
	vm.applyFilter(snapshot.Queue)
	vm.renderTable()
	vm.ensureSelection()
	row, _ := vm.table.GetSelection()
	vm.updateDetail(row)
	vm.refreshLogs(false)
	vm.renderFilter()
}

func (vm *viewModel) renderStatus(snapshot state.Snapshot) {
	if !snapshot.HasStatus {
		if snapshot.LastError != nil {
			last := "soon"
			if !snapshot.LastUpdated.IsZero() {
				last = snapshot.LastUpdated.Format("15:04:05")
			}
			vm.statusBar.SetText(fmt.Sprintf("[red]spindle unavailable[-]\nRetrying (last attempt %s)", last))
			return
		}
		vm.statusBar.SetText("[yellow]waiting for spindle status…[-]")
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
	statusText := fmt.Sprintf("[white]Daemon:[-] %s  [white]PID:[-] %d  [white]Updated:[-] %s  %s",
		yesNo(snapshot.Status.Running), snapshot.Status.PID, snapshot.LastUpdated.Format("15:04:05"), summary)
	if snapshot.LastError != nil {
		statusText += fmt.Sprintf("  [red]Error:[-] %v", snapshot.LastError)
	}
	vm.statusBar.SetText(statusText)
}

func (vm *viewModel) applyFilter(items []spindle.QueueItem) {
	if strings.TrimSpace(vm.filterText) == "" {
		vm.filteredItems = cloneItems(items)
		return
	}
	needle := strings.ToLower(vm.filterText)
	filtered := make([]spindle.QueueItem, 0, len(items))
	for _, item := range items {
		if matchItem(item, needle) {
			filtered = append(filtered, item)
		}
	}
	vm.filteredItems = filtered
}

func matchItem(item spindle.QueueItem, needle string) bool {
	fields := []string{
		item.DiscTitle,
		item.Status,
		item.ProcessingLane,
		item.Progress.Stage,
		item.Progress.Message,
		item.DiscFingerprint,
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func cloneItems(items []spindle.QueueItem) []spindle.QueueItem {
	if len(items) == 0 {
		return nil
	}
	dup := make([]spindle.QueueItem, len(items))
	copy(dup, items)
	return dup
}

func (vm *viewModel) renderTable() {
	vm.table.Clear()
	headers := []string{"ID", "Title", "Status", "Lane", "Progress"}
	for col, label := range headers {
		vm.table.SetCell(0, col, tview.NewTableCell("[::b]"+label).SetSelectable(false))
	}

	rows := vm.filteredItems
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
	rows := len(vm.filteredItems)
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

func (vm *viewModel) updateDetail(row int) {
	if row <= 0 || row-1 >= len(vm.filteredItems) {
		vm.detail.SetText("Select an item to view details")
		return
	}
	item := vm.filteredItems[row-1]
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
	if row <= 0 || row-1 >= len(vm.filteredItems) {
		return nil
	}
	item := vm.filteredItems[row-1]
	return &item
}

func (vm *viewModel) toggleFocus() {
	if vm.focusOnTable {
		vm.app.SetFocus(vm.logView)
	} else {
		vm.app.SetFocus(vm.table)
	}
	vm.focusOnTable = !vm.focusOnTable
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
	vm.refreshLogs(true)
}

func (vm *viewModel) promptFilter() {
	form := tview.NewForm()
	form.SetButtonsAlign(tview.AlignCenter)
	form.AddInputField("Match", vm.filterText, 40, nil, nil)
	form.AddButton("Apply", func() {
		item := form.GetFormItemByLabel("Match")
		ifield, ok := item.(*tview.InputField)
		if ok {
			vm.filterText = strings.TrimSpace(ifield.GetText())
		}
		vm.root.RemovePage("modal")
		vm.app.SetFocus(vm.table)
	})
	form.AddButton("Clear", func() {
		vm.filterText = ""
		vm.root.RemovePage("modal")
		vm.app.SetFocus(vm.table)
	})
	form.AddButton("Cancel", func() {
		vm.root.RemovePage("modal")
		vm.app.SetFocus(vm.table)
	})
	form.SetBorder(true).SetTitle(" Filter Queue ")

	vm.root.RemovePage("modal")
	modal := center(60, 7, form)
	vm.root.AddPage("modal", modal, true, true)
	vm.app.SetFocus(form)
}

func (vm *viewModel) renderFilter() {
	if strings.TrimSpace(vm.filterText) == "" {
		vm.filterBar.SetText(" ")
		return
	}
	vm.filterBar.SetText(fmt.Sprintf("[white]Filter:[-] %s", vm.filterText))
}

func (vm *viewModel) showHelp() {
	text := "q Quit  |  Tab Switch Focus  |  l Toggle Log  |  / Filter  |  ? Help"
	modal := tview.NewModal().SetText(text).AddButtons([]string{"Close"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		vm.root.RemovePage("modal")
		vm.app.SetFocus(vm.table)
	})
	vm.root.RemovePage("modal")
	vm.root.AddPage("modal", center(50, 7, modal), true, true)
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
