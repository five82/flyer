package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

func (vm *viewModel) activeEpisodeIndex(item spindle.QueueItem, episodes []spindle.EpisodeStatus) int {
	if len(episodes) == 0 {
		return -1
	}

	for i, ep := range episodes {
		if ep.Active {
			return i
		}
	}

	// 1. Precise Match: File path
	// If we are ripping, item.RippedFile should match episode.RippedPath (or basename)
	// If we are encoding/subtitling, item.EncodedFile should match episode.EncodedPath
	stage := normalizeEpisodeStage(item.Progress.Stage)
	if stage == "" {
		stage = normalizeEpisodeStage(item.Status)
	}

	checkMatch := func(target, candidate string) bool {
		target = strings.TrimSpace(target)
		candidate = strings.TrimSpace(candidate)
		if target == "" || candidate == "" {
			return false
		}
		// Exact match
		if target == candidate {
			return true
		}
		// Case-insensitive match on full string
		if strings.EqualFold(target, candidate) {
			return true
		}
		// Suffix match (case-insensitive)
		if strings.HasSuffix(strings.ToLower(candidate), strings.ToLower(target)) ||
			strings.HasSuffix(strings.ToLower(target), strings.ToLower(candidate)) {
			return true
		}
		// Base match
		targetBase := filepath.Base(target)
		candidateBase := filepath.Base(candidate)
		if targetBase != "." && candidateBase != "." && strings.EqualFold(targetBase, candidateBase) {
			return true
		}
		return false
	}

	if stage == "ripping" && item.RippedFile != "" {
		for i, ep := range episodes {
			if checkMatch(item.RippedFile, ep.RippedPath) || checkMatch(item.RippedFile, ep.OutputBasename) {
				return i
			}
		}
	}
	if stage == "encoding" || stage == "subtitling" {
		target := item.EncodedFile
		// Also try input match from encoding details
		if target == "" && item.Encoding != nil && item.Encoding.Video != nil {
			target = item.Encoding.Video.OutputFile
		}
		if target != "" {
			for i, ep := range episodes {
				if checkMatch(target, ep.EncodedPath) || checkMatch(target, ep.OutputBasename) {
					return i
				}
			}
		}
		// If we know the input (ripped file), we can match that too
		if item.Encoding != nil && item.Encoding.Video != nil && item.Encoding.Video.InputFile != "" {
			input := item.Encoding.Video.InputFile
			for i, ep := range episodes {
				if checkMatch(input, ep.RippedPath) {
					return i
				}
			}
		}
	}

	// 2. Stage Match: Find first episode explicitly in the current stage
	for i, ep := range episodes {
		if normalizeEpisodeStage(ep.Stage) == stage {
			return i
		}
	}

	// 3. Pipeline Match: Find first episode ready for the current stage
	// If ripping, finding first 'planned'
	// If encoding, find first 'ripped' (waiting for encoding)
	var searchStage string
	switch stage {
	case "ripping":
		searchStage = "planned"
	case "encoding":
		searchStage = "ripped"
	case "subtitling":
		searchStage = "encoded"
	}
	if searchStage != "" {
		for i, ep := range episodes {
			if normalizeEpisodeStage(ep.Stage) == searchStage {
				return i
			}
		}
	}

	// 4. Fallback: First non-final
	for i, ep := range episodes {
		if normalizeEpisodeStage(ep.Stage) != "final" && normalizeEpisodeStage(ep.Stage) != "completed" {
			return i
		}
	}

	return len(episodes) - 1
}

func (vm *viewModel) episodeStage(ep spindle.EpisodeStatus, currentGlobalStage string, isActive bool) string {
	// If the episode is active, and the global stage implies work, override it.
	if isActive {
		// Only override if global stage is a "working" stage.
		// e.g. ripping, encoding, identifying.
		switch currentGlobalStage {
		case "ripping", "encoding", "identifying", "subtitling":
			return currentGlobalStage
		}
	}
	// Otherwise use the episode's intrinsic stage.
	if ep.Stage == "final" {
		return "final"
	}
	return ep.Stage
}

func (vm *viewModel) episodeStageChip(stage string) string {
	stage = strings.ToLower(stage)
	color := vm.theme.Text.Muted
	label := "WAIT"

	switch stage {
	case "final", "completed":
		color = vm.theme.StatusColor("completed")
		label = "DONE"
	case "subtitled":
		color = vm.theme.StatusColor("subtitled")
		label = "SUB"
	case "encoded":
		color = vm.theme.StatusColor("encoded")
		label = "ENCD"
	case "ripped":
		color = vm.theme.StatusColor("ripped")
		label = "RIPD"
	case "planned":
		color = vm.theme.Text.Muted
		label = "PLAN"
	case "encoding":
		color = vm.theme.StatusColor("encoding")
		label = "WORK"
	case "ripping":
		color = vm.theme.StatusColor("ripping")
		label = "WORK"
	case "subtitling":
		color = vm.theme.StatusColor("subtitling")
		label = "WORK"
	}

	return fmt.Sprintf("[%s:%s] %s [-:-]", vm.theme.Base.Background, color, label)
}

func (vm *viewModel) describeEpisodeTotals(episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals) string {
	if len(episodes) == 0 {
		return "No episodes"
	}
	parts := []string{}
	if totals.Final > 0 {
		parts = append(parts, fmt.Sprintf("%d done", totals.Final))
	}
	if totals.Encoded > totals.Final {
		parts = append(parts, fmt.Sprintf("%d encoded", totals.Encoded-totals.Final))
	}
	if totals.Ripped > totals.Encoded {
		parts = append(parts, fmt.Sprintf("%d ripped", totals.Ripped-totals.Encoded))
	}
	rem := totals.Planned - totals.Ripped
	if rem > 0 {
		parts = append(parts, fmt.Sprintf("%d planned", rem))
	}
	return strings.Join(parts, ", ")
}

func (vm *viewModel) formatEpisodeLine(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int, active bool, stageName string) string {
	label := formatEpisodeLabel(ep)
	stage := vm.episodeStageChip(stageName)
	title, extra, _ := vm.describeEpisode(ep, titles, keyLookup)

	marker := fmt.Sprintf("[%s]·[-]", vm.theme.Text.Faint)
	if active {
		marker = fmt.Sprintf("[%s::b]>[-]", vm.theme.Text.Accent)
	}

	// Trim extras to a short set
	if len(extra) > 2 {
		extra = extra[:2]
	}
	extraText := ""
	if len(extra) > 0 {
		extraText = fmt.Sprintf(" [%s](%s)[-]", vm.theme.Text.Faint, strings.Join(extra, " · "))
	}

	titleText := fmt.Sprintf("[%s]%s[-]", vm.theme.Text.Primary, tview.Escape(title))
	if active {
		titleText = fmt.Sprintf("[%s::b]%s[-:-:-]", vm.theme.Text.Primary, tview.Escape(title))
	}

	return fmt.Sprintf("%s [%s]%s[-] %s %s%s\n", marker, vm.theme.Text.Muted, label, stage, titleText, extraText)
}

func formatEpisodeLabel(ep spindle.EpisodeStatus) string {
	if ep.Season == 0 && ep.Episode == 0 {
		return "S??E??"
	}
	return fmt.Sprintf("S%02dE%02d", ep.Season, ep.Episode)
}

func (vm *viewModel) describeEpisode(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) (string, []string, *spindle.RipSpecTitleInfo) {
	title := strings.TrimSpace(ep.Title)
	if title == "" {
		title = strings.TrimSpace(ep.OutputBasename)
	}
	if title == "" {
		title = strings.TrimSpace(ep.SourceTitle)
	}
	if title == "" {
		title = "Unlabeled"
	}
	extra := []string{}
	if runtime := formatRuntime(ep.RuntimeSeconds); runtime != "" {
		extra = append(extra, runtime)
	}
	lang := strings.TrimSpace(ep.GeneratedSubtitleLanguage)
	if lang == "" {
		lang = strings.TrimSpace(ep.SubtitleLanguage)
	}
	if lang != "" {
		extra = append(extra, strings.ToUpper(lang))
	}
	switch strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleSource)) {
	case "whisperx":
		extra = append(extra, "AI")
		switch strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleDecision)) {
		case "no_match":
			extra = append(extra, "NO-MATCH")
		case "error":
			extra = append(extra, "OS-ERR")
		}
	case "opensubtitles":
		extra = append(extra, "OS")
	}

	info := vm.lookupRipTitleInfo(ep, titles, keyLookup)
	return title, extra, info
}

func (vm *viewModel) lookupRipTitleInfo(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) *spindle.RipSpecTitleInfo {
	if ep.SourceTitleID > 0 {
		return titles[ep.SourceTitleID]
	}
	// Try key lookup
	if key, ok := keyLookup[strings.ToLower(ep.Key)]; ok {
		return titles[key]
	}
	return nil
}

func formatRuntime(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	return fmt.Sprintf("%dm", seconds/60)
}

func (vm *viewModel) describeEpisodeTrackInfo(ep *spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int) string {
	info := vm.lookupRipTitleInfo(*ep, titles, keyLookup)
	parts := []string{}
	if info != nil {
		if info.ID > 0 {
			parts = append(parts, fmt.Sprintf("Title %02d", info.ID))
		}
		if info.Duration > 0 {
			parts = append(parts, formatRuntime(info.Duration))
		}
	} else if ep.SourceTitleID > 0 {
		parts = append(parts, fmt.Sprintf("Title %02d", ep.SourceTitleID))
	}
	return strings.Join(parts, "  ")
}

func (vm *viewModel) describeEpisodeFileStates(ep *spindle.EpisodeStatus) string {
	// Simple summary
	parts := []string{}
	if ep.RippedPath != "" {
		parts = append(parts, "[+]Ripped")
	}
	if ep.EncodedPath != "" {
		parts = append(parts, "[+]Encoded")
	}
	if ep.FinalPath != "" {
		parts = append(parts, "[+]Final")
	}
	return strings.Join(parts, " ")
}

func (vm *viewModel) formatEpisodeFocusLine(ep spindle.EpisodeStatus, titles map[int]*spindle.RipSpecTitleInfo, keyLookup map[string]int, stageName string) string {
	stage := vm.episodeStageChip(stageName)
	title, _, _ := vm.describeEpisode(ep, titles, keyLookup)
	return fmt.Sprintf("%s [%s::b]%s[-:-:-]", stage, vm.theme.Text.Primary, tview.Escape(title))
}

func (vm *viewModel) movieFocusLine(summary spindle.RipSpecSummary, stage string) string {
	if len(summary.Titles) == 0 {
		return ""
	}
	main := summary.Titles[0] // Simplify for now
	name := main.Name
	if name == "" {
		name = fmt.Sprintf("Title %02d", main.ID)
	}
	stageChip := vm.episodeStageChip(stage)
	return fmt.Sprintf("%s %s", stageChip, name)
}

func (vm *viewModel) describeItemFileStates(item spindle.QueueItem, stage string) string {
	parts := []string{}
	if item.RippedFile != "" {
		parts = append(parts, "Ripped")
	}
	if item.EncodedFile != "" {
		parts = append(parts, "Encoded")
	}
	if item.FinalFile != "" {
		parts = append(parts, "Final")
	}
	return strings.Join(parts, " · ")
}

// renderEpisodesList renders the collapsible episodes list for TV shows.
func (vm *viewModel) renderEpisodesList(b *strings.Builder, item spindle.QueueItem, episodes []spindle.EpisodeStatus, totals spindle.EpisodeTotals, currentStage string, titleLookup map[int]*spindle.RipSpecTitleInfo, episodeTitleIndex map[string]int) {
	text := vm.theme.Text

	vm.writeSection(b, "Episodes")
	if !item.EpisodesSynced {
		fmt.Fprintf(b, "[%s]Episode numbers not confirmed[-]\n", text.Warning)
	}

	collapsed := vm.episodesCollapsed(item.ID)
	if collapsed {
		summary := vm.describeEpisodeTotals(episodes, totals)
		if totals.Planned > 0 {
			percent := (float64(totals.Final) / float64(totals.Planned)) * 100
			bar := vm.drawProgressBar(percent, 20, vm.theme.StatusColor("completed"))
			fmt.Fprintf(b, "[%s]%s[-]  %s %.0f%%\n", text.Primary, summary, bar, percent)
			fmt.Fprintf(b, "[%s](press t to expand)[-]\n", text.Faint)
		} else {
			fmt.Fprintf(b, "[%s]%s[-]  [%s](press t to expand)[-]\n", text.Primary, summary, text.Faint)
		}
	} else {
		activeIdx := vm.activeEpisodeIndex(item, episodes)
		for idx, ep := range episodes {
			stage := vm.episodeStage(ep, currentStage, idx == activeIdx)
			b.WriteString(vm.formatEpisodeLine(ep, titleLookup, episodeTitleIndex, idx == activeIdx, stage))
		}
		fmt.Fprintf(b, "[%s]Press t to collapse[-]\n", text.Faint)
	}
}
