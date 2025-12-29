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
		if model.root != nil && model.root.HasPage("modal") {
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
		case tcell.KeyBacktab:
			model.toggleFocusReverse()
			return nil
		case tcell.KeyEnter:
			// Toggle fullscreen for detail or log views
			focus := model.app.GetFocus()
			if focus == model.detail || focus == model.logView {
				model.toggleFullscreen()
				return nil
			}
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
				model.showProblemsView()
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
	if vm.currentView == "problems" {
		vm.refreshProblems(false)
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

			// Provide more specific error messages
			errorMsg := "Connection failed"
			errStr := snapshot.LastError.Error()
			if strings.Contains(errStr, "connection refused") {
				errorMsg = "Daemon not running"
			} else if strings.Contains(errStr, "timeout") {
				errorMsg = "Connection timeout"
			} else if strings.Contains(errStr, "no such host") {
				errorMsg = "Host not found"
			}

			vm.statusView.SetText(fmt.Sprintf("[%s::b]SPINDLE %s[-]  [%s::b]Retrying...[-]  [%s]%s[-]  [%s]logs[-] [%s]%s[-]",
				text.Danger,
				errorMsg,
				text.Warning,
				text.Muted,
				last,
				text.Faint,
				text.Secondary,
				tview.Escape(path)))
			return
		}
		vm.statusView.SetText(fmt.Sprintf("[%s::b]Connecting to Spindle...[-]", text.Warning))
		return
	}
	stats := snapshot.Status.Workflow.QueueStats
	processing := stats["identifying"] +
		stats["ripping"] +
		stats["episode_identifying"] +
		stats["episode_identified"] +
		stats["encoding"] +
		stats["subtitling"] +
		stats["subtitled"] +
		stats["organizing"]
	failed, review := countProblemCounts(snapshot.Queue)

	// Enhanced daemon status with better visual indicators
	daemonStatus := fmt.Sprintf("[%s:%s:b]● OFF [-]", text.Danger, surface)
	if snapshot.Status.Running {
		daemonStatus = fmt.Sprintf("[%s:%s:b]● ON [-]", text.Success, surface)
	}

	parts := []string{
		daemonStatus,
		fmt.Sprintf("[%s]Queue:[-] [%s]%d[-]", text.Muted, text.Secondary, len(snapshot.Queue)),
	}

	// Show active counts (processing) only if non-zero
	if processing > 0 {
		parts = append(parts, fmt.Sprintf("[%s]Active:[-] [%s]%d[-]", text.Muted, vm.colorForStatus("encoding"), processing))
	}

	failedColor := text.Danger
	if failed == 0 {
		failedColor = text.Muted
	}
	reviewColor := text.Warning
	if review == 0 {
		reviewColor = text.Muted
	}
	if compact {
		parts = append(parts, fmt.Sprintf("[%s]F:[-] [%s]%d[-]  •  [%s]R:[-] [%s]%d[-]",
			text.Muted, failedColor, failed, text.Muted, reviewColor, review))
	} else {
		parts = append(parts, fmt.Sprintf("[%s]Failed:[-] [%s]%d[-]  •  [%s]Review:[-] [%s]%d[-]",
			text.Muted, failedColor, failed, text.Muted, reviewColor, review))
	}

	// More informative timestamp with relative time
	timeSince := time.Since(snapshot.LastUpdated)
	timeStr := snapshot.LastUpdated.Format("15:04:05")
	if timeSince < time.Minute {
		timeStr += " (now)"
	} else if timeSince < time.Hour {
		timeStr += fmt.Sprintf(" (%dm ago)", int(timeSince.Minutes()))
	} else if timeSince < 24*time.Hour {
		timeStr += fmt.Sprintf(" (%dh ago)", int(timeSince.Hours()))
	}
	parts = append(parts, fmt.Sprintf("[%s]%s[-]", text.Muted, timeStr))

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

	out := strings.Join(filterStrings(parts), "  ")
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

func countProblemCounts(items []spindle.QueueItem) (failed, review int) {
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "failed" {
			failed++
			continue
		}
		if item.NeedsReview {
			review++
		}
	}
	return failed, review
}
