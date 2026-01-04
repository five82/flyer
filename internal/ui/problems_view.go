package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/five82/flyer/internal/logtail"
	"github.com/five82/flyer/internal/spindle"
)

func (vm *viewModel) refreshProblems(force bool) {
	if vm.client == nil {
		vm.problemsView.SetText("Spindle daemon unavailable")
		vm.updateProblemsTitle(nil)
		return
	}
	if !force && time.Since(vm.problems.lastSet) < LogRefreshDebounce {
		return
	}

	item := vm.selectedItem()
	if item == nil {
		vm.problemsView.SetText("Select an item to view problems")
		vm.problems.logLines = nil
		vm.problems.lastKey = ""
		vm.updateProblemsTitle(nil)
		return
	}

	key := fmt.Sprintf("problems-item-%d", item.ID)
	since := vm.problems.logCursor[key]
	req := spindle.LogQuery{
		Since:  since,
		Limit:  LogFetchLimit,
		ItemID: item.ID,
		Level:  "warn",
	}
	if since == 0 || key != vm.problems.lastKey {
		req.Tail = true
	}

	ctx := vm.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, LogFetchTimeout)
	defer cancel()

	batch, err := vm.client.FetchLogs(reqCtx, req)
	if err != nil {
		vm.problemsView.SetText(fmt.Sprintf("Error fetching problems: %v", err))
		vm.updateProblemsTitle(item)
		return
	}

	if key != vm.problems.lastKey || req.Since == 0 {
		vm.problems.logLines = nil
	}
	vm.problems.lastKey = key
	vm.problems.logCursor[key] = batch.Next

	newLines := formatLogEvents(batch.Events)
	if len(newLines) > 0 {
		vm.problems.logLines = append(vm.problems.logLines, newLines...)
		if overflow := len(vm.problems.logLines) - LogBufferLimit; overflow > 0 {
			vm.problems.logLines = append([]string(nil), vm.problems.logLines[overflow:]...)
		}
	}

	if len(vm.problems.logLines) == 0 {
		vm.problemsView.SetText("No warnings or errors for this item")
		vm.updateProblemsTitle(item)
		return
	}

	colorized := logtail.ColorizeLines(vm.problems.logLines)
	vm.displayProblems(colorized)
	vm.problems.lastSet = time.Now()
	vm.updateProblemsTitle(item)
}

func (vm *viewModel) displayProblems(colorizedLines []string) {
	numberedLines := make([]string, len(colorizedLines))
	for i, line := range colorizedLines {
		lineNum := i + 1
		numberedLines[i] = fmt.Sprintf("[%s]%4d │[-] %s", vm.theme.Text.Faint, lineNum, line)
	}
	vm.problemsView.SetText(strings.Join(numberedLines, "\n"))
	vm.problemsView.ScrollToEnd()
}

func (vm *viewModel) updateProblemsTitle(item *spindle.QueueItem) {
	if item == nil || item.ID <= 0 {
		vm.problemsView.SetTitle(" [::b]Problems[::-] ")
		return
	}
	vm.problemsView.SetTitle(fmt.Sprintf(" [::b]Problems (Item #%d)[::-] ", item.ID))
}
