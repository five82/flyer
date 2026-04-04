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

	value := "~" + formatBytes(enc.EstimatedTotalBytes)
	if enc.CurrentOutputBytes > 0 {
		value += fmt.Sprintf(" (%s written)", formatBytes(enc.CurrentOutputBytes))
	}
	renderDetailField(b, bg, "Est", styles.MutedText, value, styles.AccentText)
}

// renderSizeResult renders the file size comparison (input → output with reduction %).
func (m *Model) renderSizeResult(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil || enc.OriginalSize <= 0 || enc.EncodedSize <= 0 {
		return
	}

	value := formatBytes(enc.OriginalSize) + " -> " + formatBytes(enc.EncodedSize) + fmt.Sprintf(" (%.0f%% reduction)", enc.SizeReductionPercent)
	renderDetailField(b, bg, "Size", styles.MutedText, value, styles.Text)
}

// renderVideoSpecs renders the video specs line (resolution + HDR status).
func (m *Model) renderVideoSpecs(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	enc := item.Encoding
	if enc == nil || enc.Resolution == "" {
		return
	}
	var parts []string

	// Show cropped resolution if crop was applied, otherwise source resolution
	res := enc.Resolution
	if enc.CropRequired && enc.CropFilter != "" {
		if dims := strings.TrimPrefix(enc.CropFilter, "crop="); dims != enc.CropFilter {
			if fields := strings.SplitN(dims, ":", 3); len(fields) >= 2 {
				res = fields[0] + "x" + fields[1]
			}
		}
	}
	parts = append(parts, res)
	if enc.DynamicRange != "" {
		parts = append(parts, strings.ToUpper(enc.DynamicRange))
	}

	renderDetailField(b, bg, "Video", styles.MutedText, strings.Join(parts, " "), styles.AccentText)
}

// renderAudioInfo renders the source audio format.
func (m *Model) renderAudioInfo(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	if item.PrimaryAudioDescription == "" {
		return
	}

	renderDetailField(b, bg, "Audio", styles.MutedText, item.PrimaryAudioDescription, styles.Text)
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

	renderDetailField(b, bg, "Config", styles.MutedText, strings.Join(parts, " • "), styles.AccentText)
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

	renderDetailField(b, bg, "Encode", styles.MutedText, strings.Join(parts, " @ "), styles.Text)
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

	value := fmt.Sprintf("%d/%d checks", passed, total)
	if v.Passed {
		renderDetailField(b, bg, "Checks", styles.MutedText, "Passed · "+value, styles.SuccessText)
	} else {
		renderDetailField(b, bg, "Checks", styles.MutedText, "Failed · "+value, styles.DangerText)
	}
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
		renderDetailField(b, bg, "Crop", styles.MutedText, cropVal, styles.AccentText)
	} else if enc.CropMessage != "" {
		// Detection complete but no cropping needed
		renderDetailField(b, bg, "Crop", styles.MutedText, "None", styles.FaintText)
	}
}

// renderSubtitleSummary renders the subtitle source summary for TV shows.
func (m *Model) renderSubtitleSummary(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	episodes, _ := item.EpisodeSnapshot()
	if len(episodes) == 0 {
		return
	}

	count := 0
	for _, ep := range episodes {
		source := strings.ToLower(strings.TrimSpace(ep.GeneratedSubtitleSource))
		if source == "" {
			source = strings.ToLower(strings.TrimSpace(ep.SubtitleSource))
		}
		if source == "whisperx" {
			count++
		}
	}

	if count == 0 {
		return
	}

	renderDetailField(b, bg, "Subs", styles.MutedText, fmt.Sprintf("%d WhisperX", count), styles.AccentText)
}

// renderSubtitleInfo renders subtitle source for movies and single items.
func (m *Model) renderSubtitleInfo(b *strings.Builder, item spindle.QueueItem, styles Styles, bg BgStyle) {
	sg := item.SubtitleGeneration
	if sg == nil {
		return
	}

	if sg.WhisperX == 0 {
		return
	}

	renderDetailField(b, bg, "Subs", styles.MutedText, "WhisperX", styles.AccentText)
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
