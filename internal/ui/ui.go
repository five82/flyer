package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/config"
	"github.com/five82/flyer/internal/spindle"
	"github.com/five82/flyer/internal/state"
)

// Options configure the UI runtime.
type Options struct {
	Store         *state.Store
	Client        *spindle.Client
	Context       context.Context
	DaemonLogPath string
	Config        config.Config
	RefreshEvery  time.Duration
}

const (
	maxLogLines       = 400
	defaultUIInterval = time.Second
)

type logSource int
type queueFilter int

const (
	logSourceDaemon logSource = iota
	logSourceItem
)

const (
	filterAll queueFilter = iota
	filterFailed
	filterReview
	filterProcessing
)

// Run wires up tview components and blocks until ctx is cancelled or the user quits.
func Run(ctx context.Context, opts Options) error {
	if opts.Store == nil {
		return fmt.Errorf("ui requires a data store")
	}

	if opts.Context == nil {
		opts.Context = ctx
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
			case 'l':
				model.toggleLogSource()
				return nil
			case 'p':
				model.toggleProblemsDrawer()
				return nil
			case 'i':
				model.showItemLogsView()
				return nil
			case 'h':
				model.showHelp()
				return nil
			case '?':
				model.showHelp()
				return nil
			case 'f':
				model.cycleFilter()
				return nil
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				if model.jumpToProblem(event.Rune()) {
					return nil
				}
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

func (vm *viewModel) update(snapshot state.Snapshot) {
	vm.renderStatus(snapshot)
	vm.items = snapshot.Queue
	vm.updateProblems(snapshot.Queue)
	vm.renderTable()
	vm.ensureSelection()
	if len(vm.displayItems) > 0 {
		row, _ := vm.table.GetSelection()
		vm.updateDetail(row)
	}
	vm.lastRefresh = snapshot.LastUpdated

	if vm.currentView == "logs" {
		vm.refreshLogs(false)
	}
}

func (vm *viewModel) renderStatus(snapshot state.Snapshot) {
	text := vm.theme.Text
	surface := vm.theme.Base.Surface
	if !snapshot.HasStatus {
		if snapshot.LastError != nil {
			last := "soon"
			if !snapshot.LastUpdated.IsZero() {
				last = snapshot.LastUpdated.Format("15:04:05")
			}
			vm.statusView.SetText(fmt.Sprintf("[%s::b] SPINDLE UNAVAILABLE [-]  [%s::b]Retrying...[-] [%s]%s[-]\n[%s]Check %s or daemon logs",
				text.Danger,
				text.Warning,
				text.Muted,
				last,
				text.Faint,
				tview.Escape(vm.options.Config.DaemonLogPath())))
			return
		}
		vm.statusView.SetText(fmt.Sprintf("[%s::b] Waiting for Spindle status...[-]", text.Warning))
		return
	}
	stats := snapshot.Status.Workflow.QueueStats
	pending := stats["pending"]
	processing := stats["identifying"] + stats["ripping"] + stats["encoding"] + stats["organizing"]
	failed := stats["failed"]
	review := stats["review"]
	completed := stats["completed"]

	daemonStatus := fmt.Sprintf("[%s:%s:b] OFF [-]", text.Danger, surface)
	if snapshot.Status.Running {
		daemonStatus = fmt.Sprintf("[%s:%s:b] ON [-]", text.Success, surface)
	}

	makePill := func(label string, value int, color string) string {
		valColor := text.Secondary
		if value > 0 && color != "" {
			valColor = color
		}
		return fmt.Sprintf("[%s]%s[-] [%s]%d[-]", text.Faint, label, valColor, value)
	}

	parts := []string{
		fmt.Sprintf("DAEMON %s", daemonStatus),
		fmt.Sprintf("[%s]PID[-] [%s]%d[-]", text.Faint, text.Primary, snapshot.Status.PID),
		makePill("PEND", pending, text.Primary),
		makePill("PROC", processing, vm.colorForStatus("encoding")),
		makePill("FAIL", failed, text.Danger),
		makePill("REV", review, text.Warning),
		makePill("DONE", completed, text.Success),
	}

	statusText := strings.Join(parts, "   ")

	timeInfo := fmt.Sprintf("[%s]UPDATED[-] [%s]%s[-]", text.Faint, text.Secondary, snapshot.LastUpdated.Format("15:04:05"))
	if !vm.lastRefresh.IsZero() {
		ago := time.Since(vm.lastRefresh)
		if ago < 0 {
			ago = 0
		}
		timeInfo += fmt.Sprintf("  [%s]LAG[-] [%s]%v[-]", text.Faint, text.Secondary, ago.Round(100*time.Millisecond))
	}

	statusText += "\n" + timeInfo

	var unhealthy []string
	for _, sh := range snapshot.Status.Workflow.StageHealth {
		if !sh.Ready {
			unhealthy = append(unhealthy, fmt.Sprintf("%s: %s", sh.Name, sh.Detail))
		}
	}
	for _, dep := range snapshot.Status.Dependencies {
		if !dep.Available {
			label := dep.Name
			if dep.Detail != "" {
				label += " – " + dep.Detail
			}
			unhealthy = append(unhealthy, label)
		}
	}
	if len(unhealthy) > 0 {
		statusText += "   " + fmt.Sprintf("[%s::b]HEALTH[-] [%s]%s[-]", text.Danger, text.Danger, truncate(strings.Join(unhealthy, " | "), 90))
	}
	if snapshot.LastError != nil {
		statusText += fmt.Sprintf("   [%s::b]ERROR[-] [%s]%v[-]", text.Danger, text.Danger, snapshot.LastError)
	}
	vm.statusView.SetText(statusText)
}
