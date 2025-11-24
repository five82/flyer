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
	if !snapshot.HasStatus {
		if snapshot.LastError != nil {
			last := "soon"
			if !snapshot.LastUpdated.IsZero() {
				last = snapshot.LastUpdated.Format("15:04:05")
			}
			vm.statusView.SetText(fmt.Sprintf("[#f87171::b] SPINDLE UNAVAILABLE [-]  [#fbbf24::b]Retrying...[-] [#94a3b8]%s[-]\n[#64748b]Check %s or daemon logs", last, tview.Escape(vm.options.Config.DaemonLogPath())))
			return
		}
		vm.statusView.SetText("[#fbbf24::b] Waiting for Spindle status...[-]")
		return
	}
	stats := snapshot.Status.Workflow.QueueStats
	pending := stats["pending"]
	processing := stats["identifying"] + stats["ripping"] + stats["encoding"] + stats["organizing"]
	failed := stats["failed"]
	review := stats["review"]
	completed := stats["completed"]

	// Daemon Status Pill
	daemonStatus := "[#f87171::b] OFF [-]" // Red
	if snapshot.Status.Running {
		daemonStatus = "[#4ade80:black:b] ON [-]" // Green
	}

	// Stats Pills
	// Using Slate-700 background (#334155) for pills to stand out against black

	makePill := func(label string, value int, color string) string {
		valColor := "#e2e8f0" // Slate-200
		if value > 0 && color != "" {
			valColor = color
		}
		return fmt.Sprintf("[#94a3b8]%s[-] [%s]%d[-]", label, valColor, value)
	}

	parts := []string{
		fmt.Sprintf("DAEMON %s", daemonStatus),
		fmt.Sprintf("[#94a3b8]PID[-] [#e2e8f0]%d[-]", snapshot.Status.PID),
		makePill("PEND", pending, "#e2e8f0"),
		makePill("PROC", processing, "#38bdf8"), // Sky-400
		makePill("FAIL", failed, "#f87171"),     // Red-400
		makePill("REV", review, "#fbbf24"),      // Amber-400
		makePill("DONE", completed, "#4ade80"),  // Green-400
	}

	statusText := strings.Join(parts, "   ")

	// Right side: Timestamps
	timeInfo := fmt.Sprintf("[#64748b]UPDATED[-] [#94a3b8]%s[-]", snapshot.LastUpdated.Format("15:04:05"))
	if !vm.lastRefresh.IsZero() {
		ago := time.Since(vm.lastRefresh)
		if ago < 0 {
			ago = 0
		}
		timeInfo += fmt.Sprintf("  [#64748b]LAG[-] [#94a3b8]%v[-]", ago.Round(time.Millisecond*100))
	}

	statusText += "\n" + timeInfo

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
		statusText += "   [#f87171::b]HEALTH[-] [#f87171]" + truncate(strings.Join(unhealthy, " | "), 90) + "[-]"
	}
	if snapshot.LastError != nil {
		statusText += fmt.Sprintf("   [#f87171::b]ERROR[-] [#f87171]%v[-]", snapshot.LastError)
	}
	vm.statusView.SetText(statusText)
}
