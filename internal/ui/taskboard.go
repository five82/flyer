package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/five82/flyer/internal/spindle"
)

// renderTaskBoard renders the item's scheduler tasks: one row per task in
// pipeline order, with inline progress for running tasks. This replaces
// the old stage checklist + separate activity bar -- concurrent branches
// (rip-and-encode overlap, GPU work during encodes) each show their own
// live row.
func (m *Model) renderTaskBoard(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	if len(item.Tasks) == 0 {
		info := stageDisplay(itemDisplayStage(item))
		glyph, style := "○", styles.MutedText
		if item.IsTerminal() {
			glyph = taskStateGlyph(map[bool]string{true: "failed", false: "done"}[strings.EqualFold(item.Stage, "failed")])
			style = roleStyle(info.role, styles)
		}
		b.WriteString(bg.Spaces(2))
		b.WriteString(bg.Render(glyph, style))
		b.WriteString(bg.Space())
		b.WriteString(bg.Render(info.doneLabel, styles.Text))
		if !item.IsTerminal() {
			b.WriteString(bg.Space())
			b.WriteString(bg.Render("(tasks pending)", styles.FaintText))
		}
		b.WriteString("\n")
		return
	}

	_, totals := item.EpisodeSnapshot()
	countWidth := len(strconv.Itoa(max(totals.Planned, 1)))

	for _, task := range item.Tasks {
		m.renderTaskRow(b, item, task, totals, countWidth, styles, bg)
	}
}

func (m *Model) renderTaskRow(b *strings.Builder, item spindle.QueueItem, task spindle.Task, totals spindle.EpisodeTotals, countWidth int, styles Styles, bg BgStyle) {
	info := stageDisplay(task.Type)

	glyph := taskStateGlyph(task.State)
	var glyphStyle, labelStyle lipgloss.Style
	label := info.label

	switch task.State {
	case "done":
		glyphStyle = styles.SuccessText
		label = info.doneLabel
		labelStyle = styles.Text
	case "running":
		glyphStyle = roleStyle(info.role, styles)
		labelStyle = roleStyle(info.role, styles).Bold(true)
	case "failed":
		glyphStyle = styles.DangerText
		labelStyle = styles.DangerText.Bold(true)
	default: // pending
		glyphStyle = styles.FaintText
		labelStyle = styles.FaintText
	}

	b.WriteString(bg.Spaces(2))
	b.WriteString(bg.Render(glyph, glyphStyle))
	b.WriteString(bg.Space())
	b.WriteString(bg.Render(fmt.Sprintf("%-12s", label), labelStyle))

	// Per-episode throughput count for stages that have one (TV items).
	if count, ok := stageThroughput(info.totals, totals); ok && totals.Planned > 1 {
		b.WriteString(bg.Space())
		b.WriteString(bg.Render(fmt.Sprintf("%*d/%d", countWidth, count, totals.Planned), styles.MutedText))
	}

	switch task.State {
	case "running":
		b.WriteString(bg.Spaces(2))
		b.WriteString(m.renderProgressBar(task.Progress.Percent, 20, styles, bg))
		b.WriteString(bg.Space())
		b.WriteString(bg.Render(fmt.Sprintf("%3.0f%%", clampPercent(task.Progress.Percent)), styles.Text))
		for _, extra := range taskExtras(item, task, totals) {
			b.WriteString(bg.Spaces(2))
			b.WriteString(bg.Render(extra, styles.MutedText))
		}
	case "done":
		if d := task.Duration(); d > 0 {
			b.WriteString(bg.Spaces(2))
			b.WriteString(bg.Render(formatDuration(d), styles.FaintText))
		}
	case "failed":
		if task.Attempts > 1 {
			b.WriteString(bg.Spaces(2))
			b.WriteString(bg.Render(fmt.Sprintf("attempt %d", task.Attempts), styles.MutedText))
		}
	}
	b.WriteString("\n")

	// Second line: running message / failure error.
	switch task.State {
	case "running":
		if msg := runningTaskMessage(task); msg != "" {
			b.WriteString(bg.Spaces(6))
			b.WriteString(bg.Render(msg, styles.Text))
			b.WriteString("\n")
		}
	case "failed":
		if err := strings.TrimSpace(task.Error); err != "" {
			b.WriteString(bg.Spaces(6))
			b.WriteString(bg.Render(err, styles.DangerText))
			b.WriteString("\n")
		}
	}
}

// stageThroughput maps a catalog totals key to its tallied count.
func stageThroughput(key string, totals spindle.EpisodeTotals) (int, bool) {
	switch key {
	case "ripped":
		return totals.Ripped, true
	case "encoded":
		return totals.Encoded, true
	case "subtitled":
		return totals.Subtitled, true
	case "final":
		return totals.Final, true
	default:
		return 0, false
	}
}

// runningTaskMessage composes the running task's message line with its
// active asset key when one is set and not already in the message.
func runningTaskMessage(task spindle.Task) string {
	msg := strings.TrimSpace(task.Progress.Message)
	key := strings.TrimSpace(task.ActiveAssetKey)
	if key != "" && !strings.Contains(strings.ToLower(msg), strings.ToLower(key)) {
		if msg == "" {
			return key
		}
		return msg + " (" + key + ")"
	}
	return msg
}

// taskExtras returns supplemental figures for a running task's row:
// fps for encodes, byte progress for copy-style tasks, and an ETA.
func taskExtras(item spindle.QueueItem, task spindle.Task, totals spindle.EpisodeTotals) []string {
	var extras []string
	if task.Type == "encoding" && item.Encoding != nil && item.Encoding.FPS > 0 {
		extras = append(extras, fmt.Sprintf("%.0f fps", item.Encoding.FPS))
	}
	if task.Progress.TotalBytes > 0 {
		extras = append(extras, fmt.Sprintf("%s / %s",
			formatBytes(task.Progress.BytesCopied),
			formatBytes(task.Progress.TotalBytes)))
	}
	if eta := taskETA(item, task, totals); eta != "" {
		extras = append(extras, eta)
	}
	return extras
}

// taskETA estimates remaining time for a running task. Single-file encodes
// use reel's own ETA; everything else derives from the task's server-side
// start time and percent (no client-side stage tracking needed).
func taskETA(item spindle.QueueItem, task spindle.Task, totals spindle.EpisodeTotals) string {
	if task.Type == "encoding" && totals.Planned <= 1 && item.Encoding != nil {
		if eta := item.Encoding.ETADuration(); eta > 0 {
			return "ETA " + formatDuration(eta)
		}
	}
	percent := clampPercent(task.Progress.Percent)
	if percent < 5 || percent >= 100 {
		return ""
	}
	started := task.ParsedStartedAt()
	if started.IsZero() {
		return ""
	}
	elapsed := time.Since(started)
	if elapsed <= 0 {
		return ""
	}
	remaining := time.Duration(float64(elapsed) * (100 - percent) / percent)
	if remaining <= 0 {
		return ""
	}
	return "ETA " + formatDuration(remaining)
}

// renderProgressBar renders a text-based progress bar without percentage
// text. Callers add percentage display as needed.
func (m *Model) renderProgressBar(percent float64, width int, styles Styles, bg BgStyle) string {
	percent = clampPercent(percent)
	filled := min(int(float64(width)*percent/100), width)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bg.Render(bar, styles.AccentText)
}
