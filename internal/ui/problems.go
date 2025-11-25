package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

const (
	maxProblemShortcuts = 9
	maxProblemRows      = 6
)

type problemKind int

const (
	problemFailed problemKind = iota
	problemReview
)

type problemEntry struct {
	Item     spindle.QueueItem
	Reason   string
	Kind     problemKind
	Shortcut rune
}

func (vm *viewModel) toggleProblemsDrawer() {
	if len(vm.problemEntries) == 0 {
		vm.showNoProblemsNotice()
		return
	}
	vm.problemsOpen = !vm.problemsOpen
	vm.updateProblemDrawerHeight()
}

// jumpToProblem activates a problem entry via its numeric shortcut.
func (vm *viewModel) jumpToProblem(key rune) bool {
	if vm.searchMode {
		return false
	}
	id, ok := vm.problemShortcuts[key]
	if !ok {
		return false
	}

	// Ensure the item is visible in the queue table, then focus it.
	vm.filterMode = filterAll
	vm.renderTable()
	vm.ensureSelection()

	targetRow := vm.findRowByID(id)
	if targetRow > 0 {
		vm.table.Select(targetRow, 0)
	}
	vm.showDetailView()
	return true
}

func (vm *viewModel) findRowByID(id int64) int {
	for i, item := range vm.displayItems {
		if item.ID == id {
			return i + 1
		}
	}
	return 0
}

func (vm *viewModel) updateProblems(queue []spindle.QueueItem) {
	entries := collectProblemEntries(queue)
	vm.problemEntries = entries
	vm.problemShortcuts = map[rune]int64{}

	for i := range entries {
		if i >= maxProblemShortcuts {
			break
		}
		entries[i].Shortcut = rune('1' + i)
		vm.problemShortcuts[entries[i].Shortcut] = entries[i].Item.ID
	}

	vm.renderProblemTable(entries)
	vm.renderProblemSummary(entries)
	vm.renderProblemBar(entries)
	vm.updateProblemDrawerHeight()
}

func (vm *viewModel) renderProblemTable(entries []problemEntry) {
	vm.problemTable.Clear()

	headerColor := vm.theme.ProblemHeaderBackground()
	headerText := vm.theme.ProblemHeaderTextColor()
	headers := []struct {
		title string
		align int
	}{
		{"#", tview.AlignCenter},
		{"ID", tview.AlignRight},
		{"Status", tview.AlignLeft},
		{"Reason", tview.AlignLeft},
	}

	for col, hdr := range headers {
		cell := tview.NewTableCell(fmt.Sprintf("[%s::b]%s[-]", vm.theme.Text.Heading, hdr.title)).
			SetAlign(hdr.align).
			SetSelectable(false).
			SetBackgroundColor(headerColor).
			SetTextColor(headerText)
		vm.problemTable.SetCell(0, col, cell)
	}

	maxRows := len(entries)
	if maxRows > maxProblemRows {
		maxRows = maxProblemRows
	}

	for i := 0; i < maxRows; i++ {
		entry := entries[i]
		row := i + 1

		shortcut := ""
		if entry.Shortcut != 0 {
			shortcut = fmt.Sprintf("[%s::b]%c[-]", vm.theme.Problems.Shortcut, entry.Shortcut)
		}

		status := strings.ToUpper(entry.Item.Status)
		statusCell := fmt.Sprintf("[%s]%s[-]", vm.colorForStatus(entry.Item.Status), tview.Escape(status))

		reason := truncate(entry.Reason, 60)
		reasonColor := vm.theme.Problems.Warning
		if entry.Kind == problemFailed {
			reasonColor = vm.theme.Problems.Danger
		}

		vm.problemTable.SetCell(row, 0, vm.makeCell(shortcut, tview.AlignCenter, 1))
		vm.problemTable.SetCell(row, 1, vm.makeCell(fmt.Sprintf("[%s]%d[-]", vm.theme.Text.Muted, entry.Item.ID), tview.AlignRight, 1))
		vm.problemTable.SetCell(row, 2, vm.makeCell(statusCell, tview.AlignLeft, 1))
		vm.problemTable.SetCell(row, 3, vm.makeCell(fmt.Sprintf("[%s]%s[-]", reasonColor, tview.Escape(reason)), tview.AlignLeft, 4))
	}

	titleColor := fmt.Sprintf("%s::b", vm.theme.Text.Secondary)
	if len(entries) > 0 && entries[0].Kind == problemFailed {
		titleColor = fmt.Sprintf("%s::b", vm.theme.Text.Danger)
	}
	vm.problemTable.SetTitle(fmt.Sprintf(" [%s]Problems (%d)[::-] ", titleColor, len(entries)))
}

func (vm *viewModel) renderProblemSummary(entries []problemEntry) {
	if len(entries) == 0 {
		vm.problemSummary.SetText(fmt.Sprintf("[%s]No failed or review items.", vm.theme.Text.Muted))
		return
	}

	reasons := aggregateProblemReasons(entries)
	countFailed := 0
	countReview := 0
	for _, e := range entries {
		if e.Kind == problemFailed {
			countFailed++
		} else {
			countReview++
		}
	}

	parts := []string{}
	if countFailed > 0 {
		parts = append(parts, fmt.Sprintf("[%s]%d failed[-]", vm.theme.Text.Danger, countFailed))
	}
	if countReview > 0 {
		parts = append(parts, fmt.Sprintf("[%s]%d review[-]", vm.theme.Text.Warning, countReview))
	}

	statusLine := strings.Join(parts, "  |  ")
	if statusLine != "" {
		statusLine += "  •  "
	}

	vm.problemSummary.SetText(fmt.Sprintf("%s[%s]Reasons:[-] [%s]%s[-]    [%s](press [%s]p[-] to toggle, [%s]1-9[-] to jump)",
		statusLine,
		vm.theme.Text.Muted,
		vm.theme.Text.Secondary,
		reasons,
		vm.theme.Text.Faint,
		vm.theme.Text.AccentSoft,
		vm.theme.Text.AccentSoft))
}

// renderProblemBar surfaces a one-line ribbon when problems exist so it isn't hidden.
func (vm *viewModel) renderProblemBar(entries []problemEntry) {
	if vm.mainLayout == nil {
		return
	}
	if len(entries) == 0 {
		vm.problemBar.SetText("")
		vm.mainLayout.ResizeItem(vm.problemBar, 0, 0)
		return
	}

	reasons := aggregateProblemReasons(entries)
	countFailed := 0
	countReview := 0
	for _, e := range entries {
		if e.Kind == problemFailed {
			countFailed++
		} else {
			countReview++
		}
	}

	parts := []string{}
	if countFailed > 0 {
		parts = append(parts, fmt.Sprintf("[%s::b]%d failed[-]", vm.theme.Text.Danger, countFailed))
	}
	if countReview > 0 {
		parts = append(parts, fmt.Sprintf("[%s::b]%d review[-]", vm.theme.Text.Warning, countReview))
	}

	statusLine := strings.Join(parts, "  |  ")
	if statusLine != "" {
		statusLine += "  •  "
	}

	hint := fmt.Sprintf("[%s](p to expand, 1-9 to jump)[-]", vm.theme.Text.Faint)
	vm.problemBar.SetText(fmt.Sprintf("%s[%s]Reasons:[-] [%s]%s[-]    %s", statusLine, vm.theme.Text.Muted, vm.theme.Text.Secondary, reasons, hint))
	vm.mainLayout.ResizeItem(vm.problemBar, 1, 0)
}

func (vm *viewModel) updateProblemDrawerHeight() {
	if vm.mainLayout == nil {
		return
	}
	if len(vm.problemEntries) == 0 || !vm.problemsOpen {
		vm.mainLayout.ResizeItem(vm.problemDrawer, 0, 0)
		return
	}

	rows := len(vm.problemEntries)
	if rows > maxProblemRows {
		rows = maxProblemRows
	}
	height := rows + 2 // header + summary bar
	if height < 3 {
		height = 3
	}
	vm.mainLayout.ResizeItem(vm.problemDrawer, height, 0)
}

func collectProblemEntries(queue []spindle.QueueItem) []problemEntry {
	var entries []problemEntry
	for _, item := range queue {
		status := strings.ToLower(strings.TrimSpace(item.Status))
		isFailed := status == "failed"
		isReview := item.NeedsReview
		if !isFailed && !isReview {
			continue
		}

		kind := problemFailed
		if isReview && !isFailed {
			kind = problemReview
		}

		entries = append(entries, problemEntry{
			Item:   item,
			Reason: problemReason(item),
			Kind:   kind,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind // failed before review
		}
		ti := mostRecentTimestamp(entries[i].Item)
		tj := mostRecentTimestamp(entries[j].Item)
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return entries[i].Item.ID > entries[j].Item.ID
	})
	return entries
}

func problemReason(item spindle.QueueItem) string {
	if msg := strings.TrimSpace(item.ErrorMessage); msg != "" {
		return msg
	}
	if item.NeedsReview {
		if msg := strings.TrimSpace(item.ReviewReason); msg != "" {
			return msg
		}
	}
	if msg := strings.TrimSpace(item.Progress.Message); msg != "" {
		return msg
	}
	if stage := strings.TrimSpace(item.Progress.Stage); stage != "" {
		return titleCase(stage)
	}
	if status := strings.TrimSpace(item.Status); status != "" {
		return titleCase(status)
	}
	return "Needs attention"
}

func aggregateProblemReasons(entries []problemEntry) string {
	if len(entries) == 0 {
		return "—"
	}
	counts := map[string]int{}
	for _, entry := range entries {
		key := strings.TrimSpace(strings.ToLower(entry.Reason))
		if key == "" {
			key = "unspecified"
		}
		counts[key]++
	}

	type pair struct {
		reason string
		count  int
	}
	var pairs []pair
	for reason, count := range counts {
		pairs = append(pairs, pair{reason: reason, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].reason < pairs[j].reason
	})

	var parts []string
	for _, p := range pairs {
		label := prettifyReason(p.reason)
		parts = append(parts, fmt.Sprintf("%s ×%d", label, p.count))
	}
	return strings.Join(parts, "  |  ")
}

func prettifyReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "Unspecified"
	}

	reason = strings.ReplaceAll(reason, "_", " ")
	words := strings.Fields(reason)
	if len(words) == 0 {
		return "Unspecified"
	}

	for i, w := range words {
		lower := strings.ToLower(w)
		words[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(words, " ")
}

// showNoProblemsNotice surfaces feedback when the drawer is empty.
func (vm *viewModel) showNoProblemsNotice() {
	vm.problemSummary.SetText(fmt.Sprintf("[%s]No failed or review items.", vm.theme.Text.Muted))
	vm.problemBar.SetText("")
	vm.mainLayout.ResizeItem(vm.problemDrawer, 0, 0)
	vm.mainLayout.ResizeItem(vm.problemBar, 0, 0)
	vm.problemsOpen = false

	modal := tview.NewModal().
		SetText(fmt.Sprintf("[%s]No failed or review items to show.", vm.theme.Text.Muted)).
		AddButtons([]string{"Close"})
	modal.SetBackgroundColor(vm.theme.SurfaceColor())
	modal.SetBorderColor(vm.theme.BorderFocusColor())
	modal.SetTextColor(hexToColor(vm.theme.Text.AccentSoft))
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		vm.root.RemovePage("problems-empty")
		vm.returnToCurrentView()
	})
	vm.root.RemovePage("problems-empty")
	vm.root.AddPage("problems-empty", center(50, 5, modal), true, true)
}
