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

	// Build structured problem info from the item itself
	structuredLines := vm.buildStructuredProblems(item)

	// Combine structured problems with log lines
	if len(structuredLines) == 0 && len(vm.problems.logLines) == 0 {
		vm.problemsView.SetText("No warnings or errors for this item")
		vm.updateProblemsTitle(item)
		return
	}

	var output strings.Builder
	if len(structuredLines) > 0 {
		output.WriteString(strings.Join(structuredLines, "\n"))
	}
	if len(vm.problems.logLines) > 0 {
		if output.Len() > 0 {
			output.WriteString("\n\n")
			output.WriteString(fmt.Sprintf("[%s::b]Log Messages[-::-]\n", vm.theme.Text.Secondary))
		}
		colorized := logtail.ColorizeLines(vm.problems.logLines)
		numberedLines := make([]string, len(colorized))
		for i, line := range colorized {
			lineNum := i + 1
			numberedLines[i] = fmt.Sprintf("[%s]%4d │[-] %s", vm.theme.Text.Faint, lineNum, line)
		}
		output.WriteString(strings.Join(numberedLines, "\n"))
	}

	vm.problemsView.SetText(output.String())
	vm.problemsView.ScrollToEnd()
	vm.problems.lastSet = time.Now()
	vm.updateProblemsTitle(item)
}

// buildStructuredProblems extracts problem info from the item's structured data.
func (vm *viewModel) buildStructuredProblems(item *spindle.QueueItem) []string {
	if item == nil {
		return nil
	}

	var lines []string
	text := vm.theme.Text

	// Review reason
	if item.NeedsReview && strings.TrimSpace(item.ReviewReason) != "" {
		lines = append(lines, fmt.Sprintf("[%s::b]Review Reason[-::-]", text.Warning))
		lines = append(lines, fmt.Sprintf("  [%s]%s[-]", text.Primary, item.ReviewReason))
		lines = append(lines, "")
	}

	// Error message
	if msg := strings.TrimSpace(item.ErrorMessage); msg != "" {
		lines = append(lines, fmt.Sprintf("[%s::b]Error[-::-]", text.Danger))
		lines = append(lines, fmt.Sprintf("  [%s]%s[-]", text.Primary, msg))
		lines = append(lines, "")
	}

	// Encoding error details
	if item.Encoding != nil && item.Encoding.Error != nil {
		err := item.Encoding.Error
		if strings.TrimSpace(err.Title) != "" || strings.TrimSpace(err.Message) != "" {
			lines = append(lines, fmt.Sprintf("[%s::b]Encoding Error[-::-]", text.Danger))
			if err.Title != "" {
				lines = append(lines, fmt.Sprintf("  [%s]%s[-]", text.Primary, err.Title))
			}
			if err.Message != "" {
				lines = append(lines, fmt.Sprintf("  [%s]%s[-]", text.Secondary, err.Message))
			}
			if err.Context != "" {
				lines = append(lines, fmt.Sprintf("  [%s]Context:[-] %s", text.Muted, err.Context))
			}
			if err.Suggestion != "" {
				lines = append(lines, fmt.Sprintf("  [%s]Suggestion:[-] %s", text.Muted, err.Suggestion))
			}
			lines = append(lines, "")
		}
	}

	// Encoding warning
	if item.Encoding != nil && strings.TrimSpace(item.Encoding.Warning) != "" {
		lines = append(lines, fmt.Sprintf("[%s::b]Warning[-::-]", text.Warning))
		lines = append(lines, fmt.Sprintf("  [%s]%s[-]", text.Primary, item.Encoding.Warning))
		lines = append(lines, "")
	}

	// Validation steps
	if item.Encoding != nil && item.Encoding.Validation != nil {
		val := item.Encoding.Validation
		// Only show validation section if it failed or has steps to show
		if !val.Passed || len(val.Steps) > 0 {
			passedIcon := "✗"
			passedColor := text.Danger
			if val.Passed {
				passedIcon = "✓"
				passedColor = vm.theme.StatusColor("completed")
			}
			lines = append(lines, fmt.Sprintf("[%s::b]Validation[-::-] [%s]%s[-]", text.Secondary, passedColor, passedIcon))

			for _, step := range val.Steps {
				icon := "✗"
				color := text.Danger
				if step.Passed {
					icon = "✓"
					color = vm.theme.StatusColor("completed")
				}
				lines = append(lines, fmt.Sprintf("  [%s]%s[-] [%s]%s[-]", color, icon, text.Primary, step.Name))
				if strings.TrimSpace(step.Details) != "" {
					lines = append(lines, fmt.Sprintf("      [%s]%s[-]", text.Secondary, step.Details))
				}
			}
			lines = append(lines, "")
		}
	}

	// Trim trailing empty line
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

func (vm *viewModel) updateProblemsTitle(item *spindle.QueueItem) {
	if item == nil || item.ID <= 0 {
		vm.problemsView.SetTitle(" [::b]Problems[::-] ")
		return
	}
	vm.problemsView.SetTitle(fmt.Sprintf(" [::b]Problems (Item #%d)[::-] ", item.ID))
}
