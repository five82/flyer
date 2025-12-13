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
	logFetchLimit     = 2000
	logBufferLimit    = 5000
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
		// If a modal is open, let it handle keys (otherwise the global
		// handler swallows Esc/q and the modal can't be dismissed).
		if model.root != nil && (model.root.HasPage("modal") || model.root.HasPage("problems-empty")) {
			if event.Key() == tcell.KeyCtrlC {
				app.Stop()
				return nil
			}
			return event
		}

		// Handle queue search mode
		if model.queueSearchMode {
			switch event.Key() {
			case tcell.KeyEnter:
				model.performQueueSearch()
				return nil
			case tcell.KeyESC:
				model.cancelQueueSearch()
				return nil
			case tcell.KeyCtrlC:
				app.Stop()
				return nil
			}
			return event
		}

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
			if model.queueSearchRegex != nil {
				model.clearQueueSearch()
				return nil
			}
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
				} else {
					model.startQueueSearch()
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
			case 'd':
				model.showDetailView()
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
			case 't':
				model.toggleEpisodesCollapsed()
				return nil
			case 'P':
				model.togglePathDetail()
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
			case 'F':
				if model.currentView == "logs" {
					model.showLogFilters()
					return nil
				}
			case 'j':
				if model.currentView == "queue" && model.app.GetFocus() == model.table {
					model.moveQueueSelection(1)
					return nil
				}
			case 'k':
				if model.currentView == "queue" && model.app.GetFocus() == model.table {
					model.moveQueueSelection(-1)
					return nil
				}
			case 'g':
				if model.currentView == "queue" && model.app.GetFocus() == model.table {
					model.selectQueueTop()
					return nil
				}
			case 'G':
				if model.currentView == "queue" && model.app.GetFocus() == model.table {
					model.selectQueueBottom()
					return nil
				}
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
	vm.maybeUpdateQueueLayout()
	vm.renderStatus(snapshot)
	vm.items = snapshot.Queue
	vm.updateProblems(snapshot.Queue)
	vm.renderTablePreservingSelection()
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

	_, _, width, _ := vm.statusView.GetInnerRect()
	if width <= 0 {
		width = 120
	}
	compact := width < 100

	if !snapshot.HasStatus {
		if snapshot.LastError != nil {
			last := "soon"
			if !snapshot.LastUpdated.IsZero() {
				last = snapshot.LastUpdated.Format("15:04:05")
			}
			path := truncateMiddle(vm.options.Config.DaemonLogPath(), 50)
			vm.statusView.SetText(fmt.Sprintf("[%s::b]SPINDLE UNAVAILABLE[-]  [%s::b]Retrying...[-]  [%s]%s[-]  [%s]logs[-] [%s]%s[-]",
				text.Danger,
				text.Warning,
				text.Muted,
				last,
				text.Faint,
				text.Secondary,
				tview.Escape(path)))
			return
		}
		vm.statusView.SetText(fmt.Sprintf("[%s::b]Waiting for Spindle status...[-]", text.Warning))
		return
	}
	stats := snapshot.Status.Workflow.QueueStats
	pending := stats["pending"]
	processing := stats["identifying"] +
		stats["ripping"] +
		stats["episode_identifying"] +
		stats["episode_identified"] +
		stats["encoding"] +
		stats["subtitling"] +
		stats["subtitled"] +
		stats["organizing"]
	failed := stats["failed"]
	review := stats["review"]
	completed := stats["completed"]

	daemonStatus := fmt.Sprintf("[%s:%s:b] OFF [-]", text.Danger, surface)
	if snapshot.Status.Running {
		daemonStatus = fmt.Sprintf("[%s:%s:b] ON [-]", text.Success, surface)
	}

	makeCount := func(label string, value int, color string, always bool) string {
		if !always && value == 0 {
			return ""
		}
		valColor := text.Secondary
		if value > 0 && color != "" {
			valColor = color
		}
		return fmt.Sprintf("[%s]%s[-] [%s]%d[-]", text.Faint, label, valColor, value)
	}

	parts := []string{
		fmt.Sprintf("[%s]DAEMON[-] %s", text.Faint, daemonStatus),
		fmt.Sprintf("[%s]Q[-] [%s]%d[-]", text.Faint, text.Secondary, len(snapshot.Queue)),
	}

	if compact {
		parts = append(parts,
			makeCount("p", pending, text.Primary, false),
			makeCount("a", processing, vm.colorForStatus("encoding"), false),
			makeCount("f", failed, text.Danger, true),
			makeCount("r", review, text.Warning, true),
			makeCount("d", completed, text.Success, false),
		)
	} else {
		parts = append(parts,
			makeCount("PEND", pending, text.Primary, false),
			makeCount("PROC", processing, vm.colorForStatus("encoding"), false),
			makeCount("FAIL", failed, text.Danger, true),
			makeCount("REV", review, text.Warning, true),
			makeCount("DONE", completed, text.Success, false),
		)
	}

	timeLabel := "UPDATED"
	lagLabel := "LAG"
	if compact {
		timeLabel = "upd"
		lagLabel = "lag"
	}
	parts = append(parts, fmt.Sprintf("[%s]%s[-] [%s]%s[-]", text.Faint, timeLabel, text.Secondary, snapshot.LastUpdated.Format("15:04:05")))
	if !vm.lastRefresh.IsZero() {
		ago := time.Since(vm.lastRefresh)
		if ago < 0 {
			ago = 0
		}
		parts = append(parts, fmt.Sprintf("[%s]%s[-] [%s]%s[-]", text.Faint, lagLabel, text.Secondary, humanizeDuration(ago)))
	}

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
		detail := unhealthy[0]
		if len(unhealthy) > 1 {
			detail = fmt.Sprintf("%s +%d more", detail, len(unhealthy)-1)
		}
		max := 80
		if compact {
			max = 40
		}
		detail = truncate(detail, max)
		parts = append(parts, fmt.Sprintf("[%s::b]HEALTH[-] [%s]%s[-]", text.Danger, text.Danger, tview.Escape(detail)))
	}
	if snapshot.LastError != nil {
		maxErr := 80
		if compact {
			maxErr = 40
		}
		errText := truncate(fmt.Sprintf("%v", snapshot.LastError), maxErr)
		parts = append(parts, fmt.Sprintf("[%s::b]ERROR[-] [%s]%s[-]", text.Danger, text.Danger, tview.Escape(errText)))
	}

	sep := "  |  "
	if compact {
		sep = "  "
	}
	out := strings.Join(filterStrings(parts), sep)
	vm.statusView.SetText(out)
}

func filterStrings(values []string) []string {
	out := values[:0]
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}
