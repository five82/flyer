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
	if !force && time.Since(vm.lastProblemsSet) < 400*time.Millisecond {
		return
	}

	item := vm.selectedItem()
	if item == nil {
		vm.problemsView.SetText("Select an item to view problems")
		vm.problemsLogLines = nil
		vm.lastProblemsKey = ""
		vm.updateProblemsTitle(nil)
		return
	}

	key := fmt.Sprintf("problems-item-%d", item.ID)
	since := vm.problemsLogCursor[key]
	req := spindle.LogQuery{
		Since:  since,
		Limit:  logFetchLimit,
		ItemID: item.ID,
		Level:  "warn",
	}
	if since == 0 || key != vm.lastProblemsKey {
		req.Tail = true
	}

	ctx := vm.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	batch, err := vm.client.FetchLogs(reqCtx, req)
	if err != nil {
		vm.problemsView.SetText(fmt.Sprintf("Error fetching problems: %v", err))
		vm.updateProblemsTitle(item)
		return
	}

	if key != vm.lastProblemsKey || req.Since == 0 {
		vm.problemsLogLines = nil
	}
	vm.lastProblemsKey = key
	vm.problemsLogCursor[key] = batch.Next

	newLines := formatLogEvents(batch.Events)
	if len(newLines) > 0 {
		vm.problemsLogLines = append(vm.problemsLogLines, newLines...)
		if overflow := len(vm.problemsLogLines) - logBufferLimit; overflow > 0 {
			vm.problemsLogLines = append([]string(nil), vm.problemsLogLines[overflow:]...)
		}
	}

	if len(vm.problemsLogLines) == 0 {
		vm.problemsView.SetText("No warnings or errors for this item")
		vm.updateProblemsTitle(item)
		return
	}

	colorized := logtail.ColorizeLines(vm.problemsLogLines)
	vm.displayProblems(colorized)
	vm.lastProblemsSet = time.Now()
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
