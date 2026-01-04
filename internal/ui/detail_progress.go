package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/five82/flyer/internal/spindle"
)

func (vm *viewModel) renderActiveProgress(b *strings.Builder, item spindle.QueueItem) {
	stage := normalizeEpisodeStage(item.Progress.Stage)
	if stage == "" {
		stage = normalizeEpisodeStage(item.Status)
	}

	percent := clampPercent(item.Progress.Percent)
	label := ""
	color := ""
	icon := ""

	switch stage {
	case "ripping":
		label = "RIPPING"
		icon = "⏵"
		color = vm.theme.StatusColor("ripping")
	case "encoding":
		label = "ENCODING"
		icon = "⚙"
		color = vm.theme.StatusColor("encoding")
		// Use specific encoding percent if valid
		if enc := item.Encoding; enc != nil && enc.TotalFrames > 0 && enc.CurrentFrame > 0 {
			p := (float64(enc.CurrentFrame) / float64(enc.TotalFrames)) * 100
			if p > 0 {
				percent = p
			}
		}
	default:
		return // No active progress bar for other stages
	}

	bar := vm.drawProgressBar(percent, 30, color)
	fmt.Fprintf(b, "\n[%s::b]%s %s[::-]  %s %3.0f%%", color, icon, label, bar, percent)

	subLines := make([]string, 0, 2)
	if msg := strings.TrimSpace(item.Progress.Message); msg != "" {
		subLines = append(subLines, msg)
	}
	switch stage {
	case "encoding":
		if stats := formatEncodingMetrics(item.Encoding); stats != "" {
			subLines = append(subLines, stats)
		}
	case "ripping":
		if eta := vm.estimateETA(item); eta != "" {
			subLines = append(subLines, eta)
		}
	}
	for i, line := range subLines {
		branch := "└─"
		if i < len(subLines)-1 {
			branch = "├─"
		}
		fmt.Fprintf(b, "\n[%s]%s[-] [%s]%s[-]", vm.theme.Text.Faint, branch, vm.theme.Text.Muted, tview.Escape(line))
	}
	fmt.Fprint(b, "\n")
}

func (vm *viewModel) drawProgressBar(percent float64, width int, color string) string {
	percent = clampPercent(percent)
	// Use Unicode blocks for smoother progress display
	// █ (full), ▓ (3/4), ▒ (1/2), ░ (1/4), and empty space
	blocks := []rune{'█', '▓', '▒', '░'}

	// Calculate how many full characters we need
	fullWidth := (percent / 100.0) * float64(width)
	fullChars := int(fullWidth)

	// Calculate the fractional part for the partial block
	fraction := fullWidth - float64(fullChars)

	var bar strings.Builder

	// Add full blocks
	if fullChars > 0 {
		fmt.Fprintf(&bar, "[%s]%s[-]", color, strings.Repeat(string(blocks[0]), fullChars))
	}

	// Add partial block based on fraction
	if fullChars < width && fraction > 0 {
		partialIdx := 3 - int(fraction*4) // Map 0-1 to 3-0 for decreasing density
		if partialIdx < 0 {
			partialIdx = 0
		}
		if partialIdx > 3 {
			partialIdx = 3
		}
		fmt.Fprintf(&bar, "[%s]%c[-]", vm.theme.Text.Muted, blocks[partialIdx])
		fullChars++
	}

	// Add empty space
	remaining := width - fullChars
	if remaining > 0 {
		fmt.Fprintf(&bar, "[%s]%s[-]", vm.theme.Text.Faint, strings.Repeat("░", remaining))
	}

	return bar.String()
}

func formatEncodingMetrics(enc *spindle.EncodingStatus) string {
	if enc == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if eta := enc.ETADuration(); eta > 0 {
		parts = append(parts, fmt.Sprintf("ETA %s", humanizeDuration(eta)))
	}
	if enc.Speed > 0 {
		parts = append(parts, fmt.Sprintf("%.1fx", enc.Speed))
	}
	if enc.FPS > 0 {
		parts = append(parts, fmt.Sprintf("%.0f fps", enc.FPS))
	}
	// Frame summary
	if enc.TotalFrames > 0 && enc.CurrentFrame > 0 {
		percent := int(math.Round((float64(enc.CurrentFrame) / float64(enc.TotalFrames)) * 100))
		parts = append(parts, fmt.Sprintf("%d/%d (%d%%)", enc.CurrentFrame, enc.TotalFrames, percent))
	}
	return strings.Join(parts, " • ")
}

func (vm *viewModel) estimateETA(item spindle.QueueItem) string {
	stage := normalizeEpisodeStage(item.Progress.Stage)
	if stage == "" {
		stage = normalizeEpisodeStage(item.Status)
	}
	if enc := item.Encoding; enc != nil && (stage == "encoding" || stage == "encoded" || stage == "final") {
		if eta := enc.ETADuration(); eta > 0 {
			return "ETA " + humanizeDuration(eta)
		}
	}
	percent := clampPercent(item.Progress.Percent)
	if percent <= 1 || percent >= 100 {
		return ""
	}
	start := item.ParsedCreatedAt()
	if start.IsZero() {
		return ""
	}
	elapsed := time.Since(start)
	if elapsed <= 0 {
		return ""
	}
	remaining := time.Duration(float64(elapsed) * (100 - percent) / percent)
	if remaining <= 0 {
		return ""
	}
	return "ETA " + humanizeDuration(remaining)
}

// renderVideoSpecs renders the video specs line (resolution + HDR status).
func (vm *viewModel) renderVideoSpecs(b *strings.Builder, item spindle.QueueItem) {
	enc := item.Encoding
	if enc == nil || enc.Video == nil {
		return
	}

	video := enc.Video
	parts := []string{}

	if video.Resolution != "" {
		parts = append(parts, video.Resolution)
	}
	if video.DynamicRange != "" {
		parts = append(parts, strings.ToUpper(video.DynamicRange))
	}

	if len(parts) > 0 {
		text := vm.theme.Text
		fmt.Fprintf(b, "[%s]Video:[-]     [%s]%s[-]\n", text.Muted, text.Accent, strings.Join(parts, " "))
	}
}

// renderEncodingConfig renders the encoding config line (preset + CRF).
func (vm *viewModel) renderEncodingConfig(b *strings.Builder, item spindle.QueueItem) {
	enc := item.Encoding
	if enc == nil || enc.Config == nil {
		return
	}

	cfg := enc.Config
	parts := []string{}

	if cfg.Preset != "" {
		parts = append(parts, fmt.Sprintf("Preset %s", cfg.Preset))
	}
	if cfg.Quality != "" {
		parts = append(parts, fmt.Sprintf("CRF %s", cfg.Quality))
	}

	if len(parts) > 0 {
		text := vm.theme.Text
		fmt.Fprintf(b, "[%s]Config:[-]    [%s]%s[-]\n", text.Muted, text.Accent, strings.Join(parts, " • "))
	}
}

// renderSizeResult renders the file size comparison (input → output with reduction %).
func (vm *viewModel) renderSizeResult(b *strings.Builder, item spindle.QueueItem) {
	enc := item.Encoding
	if enc == nil || enc.Result == nil {
		return
	}

	r := enc.Result
	if r.OriginalSize <= 0 || r.EncodedSize <= 0 {
		return
	}

	text := vm.theme.Text
	fmt.Fprintf(b, "[%s]Size:[-]      [%s]%s[-] [%s]→[-] [%s]%s[-] [%s](%.0f%% reduction)[-]\n",
		text.Muted,
		text.Secondary, formatBytes(r.OriginalSize),
		text.Faint,
		text.Accent, formatBytes(r.EncodedSize),
		text.Muted, r.SizeReductionPercent)
}

// renderSubtitleSummary renders the subtitle source summary (X OpenSubtitles • Y WhisperX).
func (vm *viewModel) renderSubtitleSummary(b *strings.Builder, item spindle.QueueItem) {
	episodes, _ := item.EpisodeSnapshot()
	if len(episodes) == 0 {
		return
	}

	osCount := 0
	aiCount := 0
	for _, ep := range episodes {
		source := strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleSource))
		if source == "" {
			source = strings.ToLower(strings.TrimSpace(ep.SubtitleSource))
		}
		switch source {
		case "opensubtitles":
			osCount++
		case "whisperx":
			aiCount++
		}
	}

	if osCount == 0 && aiCount == 0 {
		return
	}

	parts := []string{}
	if osCount > 0 {
		parts = append(parts, fmt.Sprintf("%d OpenSubtitles", osCount))
	}
	if aiCount > 0 {
		parts = append(parts, fmt.Sprintf("%d WhisperX", aiCount))
	}

	text := vm.theme.Text
	fmt.Fprintf(b, "[%s]Subtitles:[-] [%s]%s[-]\n", text.Muted, text.Accent, strings.Join(parts, " • "))
}

// renderPathsExpanded renders paths without truncation (for failed items).
func (vm *viewModel) renderPathsExpanded(b *strings.Builder, item spindle.QueueItem) {
	text := vm.theme.Text

	writePath := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		fmt.Fprintf(b, "[%s]%s[-]  [%s]%s[-]\n", text.Muted, padRight(label, 8), text.Secondary, tview.Escape(value))
	}

	writePath("Source:", item.SourcePath)
	writePath("Log:", item.ItemLogPath)
}
