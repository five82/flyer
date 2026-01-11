package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

// renderSizeResult renders the file size comparison (input → output with reduction %).
func (m *Model) renderSizeResult(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil || enc.Result == nil {
		return
	}
	r := enc.Result
	if r.OriginalSize <= 0 || r.EncodedSize <= 0 {
		return
	}

	b.WriteString(bg.Render("Size:      ", styles.MutedText))
	b.WriteString(bg.Render(formatBytes(r.OriginalSize), styles.Text))
	b.WriteString(bg.Render(" → ", styles.FaintText))
	b.WriteString(bg.Render(formatBytes(r.EncodedSize), styles.AccentText))
	b.WriteString(bg.Render(fmt.Sprintf(" (%.0f%% reduction)", r.SizeReductionPercent), styles.MutedText))
	b.WriteString("\n")
}

// renderVideoSpecs renders the video specs line (resolution + HDR status).
func (m *Model) renderVideoSpecs(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
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
		b.WriteString(bg.Render("Video:     ", styles.MutedText))
		b.WriteString(bg.Render(strings.Join(parts, " "), styles.AccentText))
		b.WriteString("\n")
	}
}

// renderAudioInfo renders the source audio format and commentary count.
func (m *Model) renderAudioInfo(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	if item.PrimaryAudioDescription == "" {
		return
	}

	audio := item.PrimaryAudioDescription
	if item.CommentaryCount > 0 {
		audio = fmt.Sprintf("%s + %d commentary", audio, item.CommentaryCount)
	}

	b.WriteString(bg.Render("Audio:     ", styles.MutedText))
	b.WriteString(bg.Render(audio, styles.Text))
	b.WriteString("\n")
}

// renderEncodingConfig renders the encoding config line (preset + CRF + tune).
func (m *Model) renderEncodingConfig(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
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
		parts = append(parts, cfg.Quality)
	}
	if cfg.Tune != "" {
		parts = append(parts, fmt.Sprintf("Tune %s", cfg.Tune))
	}

	if len(parts) > 0 {
		b.WriteString(bg.Render("Config:    ", styles.MutedText))
		b.WriteString(bg.Render(strings.Join(parts, " • "), styles.AccentText))
		b.WriteString("\n")
	}
}

// renderEncodeStats renders duration and average speed (for completed).
func (m *Model) renderEncodeStats(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil || enc.Result == nil {
		return
	}
	r := enc.Result
	if r.DurationSeconds <= 0 && r.AverageSpeed <= 0 {
		return
	}

	parts := []string{}
	if r.DurationSeconds > 0 {
		dur := time.Duration(r.DurationSeconds * float64(time.Second))
		parts = append(parts, humanizeDurationLong(dur))
	}
	if r.AverageSpeed > 0 {
		parts = append(parts, fmt.Sprintf("%.1fx avg", r.AverageSpeed))
	}

	b.WriteString(bg.Render("Encoded:   ", styles.MutedText))
	b.WriteString(bg.Render(strings.Join(parts, " @ "), styles.Text))
	b.WriteString("\n")
}

// renderValidationSummary renders a one-line validation summary for completed items.
func (m *Model) renderValidationSummary(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	if item.Encoding == nil || item.Encoding.Validation == nil {
		return
	}
	v := item.Encoding.Validation
	total := len(v.Steps)
	if total == 0 {
		return
	}

	passed := 0
	for _, step := range v.Steps {
		if step.Passed {
			passed++
		}
	}

	b.WriteString(bg.Render("Validation:", styles.MutedText) + bg.Space())
	if v.Passed {
		b.WriteString(bg.Render("✓ Passed", styles.SuccessText))
		b.WriteString(bg.Render(fmt.Sprintf(" (%d/%d checks)", passed, total), styles.FaintText))
	} else {
		b.WriteString(bg.Render("✗ Failed", styles.DangerText))
		b.WriteString(bg.Render(fmt.Sprintf(" (%d/%d checks)", passed, total), styles.FaintText))
	}
	b.WriteString("\n")
}

// renderCropInfo renders the crop detection line.
func (m *Model) renderCropInfo(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil || enc.Crop == nil {
		return
	}
	crop := enc.Crop

	if crop.Disabled {
		b.WriteString(bg.Render("Crop:      ", styles.MutedText))
		b.WriteString(bg.Render("Disabled", styles.FaintText))
		b.WriteString("\n")
		return
	}

	if crop.Required && crop.Crop != "" {
		// Strip "crop=" prefix for cleaner display
		cropVal := strings.TrimPrefix(crop.Crop, "crop=")
		b.WriteString(bg.Render("Crop:      ", styles.MutedText))
		b.WriteString(bg.Render(cropVal, styles.AccentText))
		b.WriteString("\n")
	} else if crop.Message != "" {
		// Detection complete but no cropping needed
		b.WriteString(bg.Render("Crop:      ", styles.MutedText))
		b.WriteString(bg.Render("None", styles.FaintText))
		b.WriteString("\n")
	}
}

// renderSubtitleSummary renders the subtitle source summary for TV shows.
func (m *Model) renderSubtitleSummary(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
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

	b.WriteString(bg.Render("Subs:      ", styles.MutedText))
	b.WriteString(bg.Render(strings.Join(parts, " • "), styles.AccentText))
	b.WriteString("\n")
}

// renderValidationDetails renders detailed validation step results for failed items.
func (m *Model) renderValidationDetails(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	if item.Encoding == nil || item.Encoding.Validation == nil {
		return
	}
	v := item.Encoding.Validation
	if len(v.Steps) == 0 {
		return
	}
	// Only show details if validation failed or if there are any failed steps
	hasFailures := !v.Passed
	if !hasFailures {
		for _, step := range v.Steps {
			if !step.Passed {
				hasFailures = true
				break
			}
		}
	}
	if !hasFailures {
		return
	}

	m.writeSection(b, "Validation", styles, bg)

	for _, step := range v.Steps {
		icon := "✓"
		iconStyle := styles.SuccessText
		if !step.Passed {
			icon = "✗"
			iconStyle = styles.DangerText
		}
		name := strings.TrimSpace(step.Name)
		if name == "" {
			name = "Check"
		}
		b.WriteString(bg.Render(icon, iconStyle))
		b.WriteString(bg.Space())
		b.WriteString(bg.Render(name, styles.Text))
		if details := strings.TrimSpace(step.Details); details != "" {
			b.WriteString(bg.Space())
			b.WriteString(bg.Render(details, styles.FaintText))
		}
		b.WriteString("\n")
	}
}

// renderMetadata renders metadata rows.
func (m *Model) renderMetadata(b *strings.Builder, rows []metadataRow, styles Styles, bg BgStyle) {
	for _, r := range rows {
		key := prettifyMetaKey(r.key)
		b.WriteString(bg.Spaces(2))
		b.WriteString(bg.Render(key+":", styles.MutedText))
		b.WriteString(bg.Space())
		b.WriteString(bg.Render(truncate(r.value, 60), styles.AccentText))
		b.WriteString("\n")
	}
}
