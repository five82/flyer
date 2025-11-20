package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/config"
	"github.com/five82/flyer/internal/state"
)

// Options configure the UI runtime.
type Options struct {
	Store         *state.Store
	DaemonLogPath string
	DraptoLogPath string
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
	logSourceEncoding
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
			case 'r':
				model.showEncodingLogsView()
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
	vm.lastRefresh = snapshot.LastUpdated

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
			vm.statusView.SetText(fmt.Sprintf("[red::b]spindle unavailable[-]  [magenta::b]retrying[-] [orange]%s[-]\n[gray]Check %s or daemon logs", last, tview.Escape(vm.options.Config.DaemonLogPath())))
			return
		}
		vm.statusView.SetText("[yellow::b]waiting for spindle status…[-]")
		return
	}
	stats := snapshot.Status.Workflow.QueueStats
	pending := stats["pending"]
	processing := stats["identifying"] + stats["ripping"] + stats["encoding"] + stats["organizing"]
	failed := stats["failed"]
	review := stats["review"]
	completed := stats["completed"]

	// Simplified daemon status
	daemonStatus := "[red::b]no[-]"
	if snapshot.Status.Running {
		daemonStatus = "[lightgreen::b]yes[-]" // pleasing green
	}

	// Color code values based on conditions
	failedColor := "[lightgray]"
	if failed > 0 {
		failedColor = "[red]"
	}

	reviewColor := "[lightgray]"
	if review > 0 {
		reviewColor = "[yellow]"
	}

	parts := []string{
		fmt.Sprintf("[mediumpurple]daemon[-] %s", daemonStatus),
		fmt.Sprintf("[mediumpurple]pid[-] [lightgray]%d[-]", snapshot.Status.PID),
		fmt.Sprintf("[mediumpurple]pending[-] [lightgray]%d[-]", pending),
		fmt.Sprintf("[mediumpurple]proc[-] [lightgray]%d[-]", processing),
		fmt.Sprintf("[mediumpurple]fail[-] %s%d[-]", failedColor, failed),
		fmt.Sprintf("[mediumpurple]review[-] %s%d[-]", reviewColor, review),
		fmt.Sprintf("[mediumpurple]done[-] [lightgray]%d[-]", completed),
		fmt.Sprintf("[mediumpurple]updated[-] [lightgray]%s[-]", snapshot.LastUpdated.Format("15:04:05")),
	}
	if vm.options.RefreshEvery > 0 {
		parts = append(parts, fmt.Sprintf("[mediumpurple]poll[-] [lightgray]%s[-]", vm.options.RefreshEvery))
	}
	if !vm.lastRefresh.IsZero() {
		ago := time.Since(vm.lastRefresh)
		if ago < 0 {
			ago = 0
		}
		parts = append(parts, fmt.Sprintf("[mediumpurple]last[-] [lightgray]%s ago[-]", humanizeDuration(ago)))
	}
	statusText := strings.Join(parts, "  │  ")

	// Stage / dependency health summary
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
		statusText += "\n[orangered::b]health[-] [red]" + truncate(strings.Join(unhealthy, " | "), 90) + "[-]"
	}
	if snapshot.LastError != nil {
		statusText += fmt.Sprintf("\n[white::b]error[-] [red]%v[-]", snapshot.LastError)
	}
	vm.statusView.SetText(statusText)
}
