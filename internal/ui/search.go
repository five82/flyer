package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/logtail"
)

func (vm *viewModel) startSearch() {
	if vm.currentView != "logs" {
		return
	}

	vm.search.mode = true
	vm.search.input = tview.NewInputField()
	vm.search.input.SetLabel("/")
	vm.search.input.SetFieldWidth(40)
	vm.search.input.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.search.input.SetFieldBackgroundColor(vm.theme.SurfaceAltColor())
	vm.search.input.SetFieldTextColor(hexToColor(vm.theme.Text.Primary))

	vm.search.hint = tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	vm.search.hint.SetBackgroundColor(vm.theme.SurfaceColor())
	vm.search.hint.SetTextColor(hexToColor(vm.theme.Text.Muted))
	vm.search.hint.SetText(fmt.Sprintf("[%s]Enter to search (regex, case-insensitive). Esc to cancel.[-]", vm.theme.Text.Muted))

	vm.search.input.SetChangedFunc(func(_ string) {
		if vm.search.hint != nil {
			vm.search.hint.SetText(fmt.Sprintf("[%s]Enter to search (regex, case-insensitive). Esc to cancel.[-]", vm.theme.Text.Muted))
		}
	})

	// Create a simple container for the search input
	searchContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	searchContainer.SetBackgroundColor(vm.theme.SurfaceColor())
	searchContainer.AddItem(nil, 0, 1, false) // Push to bottom
	searchContainer.AddItem(vm.search.hint, 1, 0, false)
	searchContainer.AddItem(vm.search.input, 1, 0, true)

	vm.search.input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			vm.performSearch()
		case tcell.KeyESC:
			vm.cancelSearch()
		}
	})

	vm.root.AddPage("search", searchContainer, true, true)
	vm.app.SetFocus(vm.search.input)
}

func (vm *viewModel) performSearch() {
	if vm.search.input == nil {
		return
	}
	searchText := strings.TrimSpace(vm.search.input.GetText())
	if searchText == "" {
		vm.cancelSearch()
		return
	}

	// Compile regex for case-insensitive search
	regex, err := regexp.Compile("(?i)" + searchText)
	if err != nil {
		if vm.search.hint != nil {
			vm.search.hint.SetText(fmt.Sprintf("[%s]Invalid regex: %s[-]", vm.theme.Search.Error, tview.Escape(err.Error())))
		}
		return
	}

	vm.search.regex = regex
	vm.search.lastPattern = searchText
	vm.root.RemovePage("search")
	vm.search.mode = false
	vm.search.hint = nil

	// Find matches in current log content
	vm.findSearchMatches()
	if len(vm.search.matches) > 0 {
		vm.search.currentMatch = 0
		vm.highlightSearchMatch()
		vm.updateSearchStatus()
	} else {
		vm.search.status.SetText(fmt.Sprintf("[%s]Pattern not found: %s[-]", vm.theme.Search.Error, searchText))
	}
}

func (vm *viewModel) cancelSearch() {
	vm.root.RemovePage("search")
	vm.search.mode = false
	vm.clearSearch()
	vm.search.hint = nil
	vm.returnToCurrentView()
	vm.updateLogStatus(vm.logs.follow, vm.logs.lastPath)
}

func (vm *viewModel) clearSearch() {
	vm.search.regex = nil
	vm.search.matches = []int{}
	vm.search.currentMatch = 0
	vm.search.lastPattern = ""
	vm.search.status.SetText("")
	vm.updateLogStatus(vm.logs.follow, vm.logs.lastPath)
}

func (vm *viewModel) updateSearchStatus() {
	if vm.search.regex == nil || len(vm.search.matches) == 0 {
		vm.search.status.SetText("")
		return
	}

	matchNum := vm.search.currentMatch + 1
	totalMatches := len(vm.search.matches)
	vm.search.status.SetText(fmt.Sprintf("[%s]/%s[-] - [%s]%d/%d[-] - Press [%s]n[-] for next, [%s]N[-] for previous",
		vm.theme.Search.Prompt,
		vm.search.lastPattern,
		vm.theme.Search.Count,
		matchNum,
		totalMatches,
		vm.theme.Search.Match,
		vm.theme.Search.Match))
}

func (vm *viewModel) findSearchMatches() {
	if vm.search.regex == nil {
		return
	}

	lines := vm.logs.rawLines
	vm.search.matches = []int{}
	for i, line := range lines {
		if vm.search.regex.MatchString(line) {
			vm.search.matches = append(vm.search.matches, i)
		}
	}
}

func (vm *viewModel) nextSearchMatch() {
	if len(vm.search.matches) == 0 {
		return
	}

	vm.search.currentMatch = (vm.search.currentMatch + 1) % len(vm.search.matches)
	vm.highlightSearchMatch()
	vm.updateSearchStatus()
}

func (vm *viewModel) previousSearchMatch() {
	if len(vm.search.matches) == 0 {
		return
	}

	vm.search.currentMatch = (vm.search.currentMatch - 1 + len(vm.search.matches)) % len(vm.search.matches)
	vm.highlightSearchMatch()
	vm.updateSearchStatus()
}

func (vm *viewModel) highlightSearchMatch() {
	if len(vm.search.matches) == 0 || vm.search.currentMatch >= len(vm.search.matches) {
		return
	}

	targetLine := vm.search.matches[vm.search.currentMatch]

	highlighted := make([]string, len(vm.logs.rawLines))
	for i, line := range vm.logs.rawLines {
		colored := logtail.ColorizeLine(line)
		if vm.search.regex.MatchString(line) {
			if i == targetLine {
				colored = fmt.Sprintf("[%s:%s]%s[-:-]", vm.theme.Search.HighlightActiveFg, vm.theme.Search.HighlightActiveBg, colored)
			} else {
				colored = fmt.Sprintf("[%s]%s[-]", vm.theme.Search.HighlightPassiveFg, colored)
			}
		}
		highlighted[i] = colored
	}
	vm.logView.SetText(strings.Join(highlighted, "\n"))
	vm.logView.ScrollTo(targetLine, 0)
}
