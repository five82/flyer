package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

// renderEstimatedSize renders the estimated output size during encoding.
// Only displays when progress >= 10% for estimate accuracy.
func (m *Model) renderEstimatedSize(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil {
		return
	}
	// Only show after 10% progress for accuracy
	if enc.Percent < 10 {
		return
	}
	if enc.EstimatedTotalBytes <= 0 {
		return
	}

	b.WriteString(bg.Render("Est. size: ", styles.MutedText))
	b.WriteString(bg.Render("~"+formatBytes(enc.EstimatedTotalBytes), styles.AccentText))
	if enc.CurrentOutputBytes > 0 {
		b.WriteString(bg.Render(fmt.Sprintf(" (%s written)", formatBytes(enc.CurrentOutputBytes)), styles.FaintText))
	}
	b.WriteString("\n")
}

// renderSizeResult renders the file size comparison (input → output with reduction %).
func (m *Model) renderSizeResult(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil || enc.OriginalSize <= 0 || enc.EncodedSize <= 0 {
		return
	}

	b.WriteString(bg.Render("Size:      ", styles.MutedText))
	b.WriteString(bg.Render(formatBytes(enc.OriginalSize), styles.Text))
	b.WriteString(bg.Render(" → ", styles.FaintText))
	b.WriteString(bg.Render(formatBytes(enc.EncodedSize), styles.AccentText))
	b.WriteString(bg.Render(fmt.Sprintf(" (%.0f%% reduction)", enc.SizeReductionPercent), styles.MutedText))
	b.WriteString("\n")
}

// renderVideoSpecs renders the video specs line (resolution + HDR status).
func (m *Model) renderVideoSpecs(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil || enc.Resolution == "" {
		return
	}
	var parts []string

	parts = append(parts, enc.Resolution)
	if enc.DynamicRange != "" {
		parts = append(parts, strings.ToUpper(enc.DynamicRange))
	}

	b.WriteString(bg.Render("Video:     ", styles.MutedText))
	b.WriteString(bg.Render(strings.Join(parts, " "), styles.AccentText))
	b.WriteString("\n")
}

// renderAudioInfo renders the source audio format.
func (m *Model) renderAudioInfo(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	if item.PrimaryAudioDescription == "" {
		return
	}

	b.WriteString(bg.Render("Audio:     ", styles.MutedText))
	b.WriteString(bg.Render(item.PrimaryAudioDescription, styles.Text))
	b.WriteString("\n")
}

// renderEncodingConfig renders the encoding config line (preset + CRF + tune).
func (m *Model) renderEncodingConfig(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil || enc.Preset == "" {
		return
	}
	var parts []string

	parts = append(parts, fmt.Sprintf("Preset %s", enc.Preset))
	if enc.Quality != "" {
		parts = append(parts, enc.Quality)
	}
	if enc.Tune != "" {
		parts = append(parts, fmt.Sprintf("Tune %s", enc.Tune))
	}

	b.WriteString(bg.Render("Config:    ", styles.MutedText))
	b.WriteString(bg.Render(strings.Join(parts, " • "), styles.AccentText))
	b.WriteString("\n")
}

// renderEncodeStats renders duration and average speed (for completed).
func (m *Model) renderEncodeStats(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil || (enc.EncodeDurationSeconds <= 0 && enc.AverageSpeed <= 0) {
		return
	}

	var parts []string
	if enc.EncodeDurationSeconds > 0 {
		dur := time.Duration(enc.EncodeDurationSeconds * float64(time.Second))
		parts = append(parts, humanizeDurationLong(dur))
	}
	if enc.AverageSpeed > 0 {
		parts = append(parts, fmt.Sprintf("%.1fx avg", enc.AverageSpeed))
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
	} else {
		b.WriteString(bg.Render("✗ Failed", styles.DangerText))
	}
	b.WriteString(bg.Render(fmt.Sprintf(" (%d/%d checks)", passed, total), styles.FaintText))
	b.WriteString("\n")
}

// renderCropInfo renders the crop detection line.
func (m *Model) renderCropInfo(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil {
		return
	}

	if enc.CropRequired && enc.CropFilter != "" {
		// Strip "crop=" prefix for cleaner display
		cropVal := strings.TrimPrefix(enc.CropFilter, "crop=")
		b.WriteString(bg.Render("Crop:      ", styles.MutedText))
		b.WriteString(bg.Render(cropVal, styles.AccentText))
		b.WriteString("\n")
	} else if enc.CropMessage != "" {
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

	var parts []string
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

// renderSubtitleInfo renders subtitle source for movies and single items.
func (m *Model) renderSubtitleInfo(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	sg := item.SubtitleGeneration
	if sg == nil {
		return
	}

	// Determine the source used
	var source string
	if sg.OpenSubtitles > 0 {
		source = "OpenSubtitles"
	} else if sg.WhisperX > 0 {
		source = "WhisperX"
		if sg.FallbackUsed {
			source += " (fallback)"
		}
	}

	if source == "" {
		return
	}

	b.WriteString(bg.Render("Subs:      ", styles.MutedText))
	b.WriteString(bg.Render(source, styles.AccentText))
	b.WriteString("\n")
}

// renderValidationDetails renders detailed validation step results for failed items.
func (m *Model) renderValidationDetails(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	if item.Encoding == nil || item.Encoding.Validation == nil {
		return
	}
	v := item.Encoding.Validation
	if len(v.Steps) == 0 || v.Passed {
		return
	}

	m.writeSection(b, "Validation", styles, bg)

	for _, step := range v.Steps {
		icon, iconStyle := "✓", styles.SuccessText
		if !step.Passed {
			icon, iconStyle = "✗", styles.DangerText
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
		key := titleCase(r.key)
		b.WriteString(bg.Spaces(2))
		b.WriteString(bg.Render(key+":", styles.MutedText))
		b.WriteString(bg.Space())
		b.WriteString(bg.Render(truncate(r.value, 60), styles.AccentText))
		b.WriteString("\n")
	}
}
