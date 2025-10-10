package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (vm *viewModel) startSearch() {
	if vm.currentView != "logs" {
		return
	}

	vm.searchMode = true
	vm.searchInput = tview.NewInputField()
	vm.searchInput.SetLabel("/")
	vm.searchInput.SetFieldWidth(40)
	vm.searchInput.SetBackgroundColor(tcell.ColorBlack)
	vm.searchInput.SetFieldTextColor(tcell.ColorWhite)

	// Create a simple container for the search input
	searchContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	searchContainer.SetBackgroundColor(tcell.ColorBlack)
	searchContainer.AddItem(nil, 0, 1, false) // Push to bottom
	searchContainer.AddItem(vm.searchInput, 1, 0, true)

	vm.searchInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			vm.performSearch()
		case tcell.KeyESC:
			vm.cancelSearch()
		}
	})

	vm.root.AddPage("search", searchContainer, true, true)
	vm.app.SetFocus(vm.searchInput)
}

func (vm *viewModel) performSearch() {
	if vm.searchInput == nil {
		return
	}
	searchText := strings.TrimSpace(vm.searchInput.GetText())
	if searchText == "" {
		vm.cancelSearch()
		return
	}

	// Compile regex for case-insensitive search
	regex, err := regexp.Compile("(?i)" + searchText)
	if err != nil {
		vm.cancelSearch()
		return
	}

	vm.searchRegex = regex
	vm.lastSearchPattern = searchText
	vm.root.RemovePage("search")
	vm.searchMode = false

	// Find matches in current log content
	vm.findSearchMatches()
	if len(vm.searchMatches) > 0 {
		vm.currentSearchMatch = 0
		vm.highlightSearchMatch()
		vm.updateSearchStatus()
	} else {
		vm.searchStatus.SetText(fmt.Sprintf("[red]Pattern not found: %s[-]", searchText))
	}
}

func (vm *viewModel) cancelSearch() {
	vm.root.RemovePage("search")
	vm.searchMode = false
	vm.returnToCurrentView()
}

func (vm *viewModel) clearSearch() {
	vm.searchRegex = nil
	vm.searchMatches = []int{}
	vm.currentSearchMatch = 0
	vm.lastSearchPattern = ""
	vm.searchStatus.SetText("")
}

func (vm *viewModel) updateSearchStatus() {
	if vm.searchRegex == nil || len(vm.searchMatches) == 0 {
		vm.searchStatus.SetText("")
		return
	}

	matchNum := vm.currentSearchMatch + 1
	totalMatches := len(vm.searchMatches)
	vm.searchStatus.SetText(fmt.Sprintf("[dodgerblue]/%s[-] - [yellow]%d/%d[-] - Press [dodgerblue]n[-] for next, [dodgerblue]N[-] for previous",
		vm.lastSearchPattern, matchNum, totalMatches))
}

func (vm *viewModel) findSearchMatches() {
	if vm.searchRegex == nil {
		return
	}

	logText := vm.logView.GetText(false)
	lines := strings.Split(logText, "\n")

	vm.searchMatches = []int{}
	for i, line := range lines {
		if vm.searchRegex.MatchString(line) {
			vm.searchMatches = append(vm.searchMatches, i)
		}
	}
}

func (vm *viewModel) nextSearchMatch() {
	if len(vm.searchMatches) == 0 {
		return
	}

	vm.currentSearchMatch = (vm.currentSearchMatch + 1) % len(vm.searchMatches)
	vm.highlightSearchMatch()
	vm.updateSearchStatus()
}

func (vm *viewModel) previousSearchMatch() {
	if len(vm.searchMatches) == 0 {
		return
	}

	vm.currentSearchMatch = (vm.currentSearchMatch - 1 + len(vm.searchMatches)) % len(vm.searchMatches)
	vm.highlightSearchMatch()
	vm.updateSearchStatus()
}

func (vm *viewModel) highlightSearchMatch() {
	if len(vm.searchMatches) == 0 || vm.currentSearchMatch >= len(vm.searchMatches) {
		return
	}

	targetLine := vm.searchMatches[vm.currentSearchMatch]

	// Get original log content (without highlighting)
	logText := vm.logView.GetText(false)
	lines := strings.Split(logText, "\n")

	// Highlight all matches, but emphasize the current one
	for i, line := range lines {
		if vm.searchRegex.MatchString(line) {
			if i == targetLine {
				// Current match: yellow background with bold text
				lines[i] = vm.searchRegex.ReplaceAllString(line, "[::b][black:yellow]${0}[-]")
			} else {
				// Other matches: just highlight in red
				lines[i] = vm.searchRegex.ReplaceAllString(line, "[red]${0}[-]")
			}
		}
	}

	// Update the log view with highlighted content
	vm.logView.SetText(strings.Join(lines, "\n"))

	// Scroll to the matched line
	vm.logView.ScrollTo(targetLine, 0)
}
