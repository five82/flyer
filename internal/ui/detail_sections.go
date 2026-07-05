package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

// renderEstimatedSize renders the estimated output size during encoding.
// Only displays when progress >= 10% for estimate accuracy.
func renderEstimatedSize(w fieldWriter, item spindle.QueueItem) {
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
	// Once the final size is known the estimate is stale noise.
	if enc.EncodedSize > 0 {
		return
	}

	value := "~" + formatBytes(enc.EstimatedTotalBytes)
	if enc.CurrentOutputBytes > 0 {
		value += fmt.Sprintf(" (%s written)", formatBytes(enc.CurrentOutputBytes))
	}
	w.field("Est", value, w.styles.AccentText)
}

// renderSizeResult renders the file size comparison (input -> output with reduction %).
func renderSizeResult(w fieldWriter, item spindle.QueueItem) {
	enc := item.Encoding
	if enc == nil || enc.OriginalSize <= 0 || enc.EncodedSize <= 0 {
		return
	}

	value := formatBytes(enc.OriginalSize) + " -> " + formatBytes(enc.EncodedSize) +
		fmt.Sprintf(" (%.0f%% reduction)", enc.SizeReductionPercent)
	w.field("Size", value, w.styles.Text)
}

// renderVideoSpecs renders the video specs line (resolution + HDR status).
func renderVideoSpecs(w fieldWriter, item spindle.QueueItem) {
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

	w.field("Video", strings.Join(parts, " "), w.styles.AccentText)
}

// renderAudioInfo renders the source audio format.
func renderAudioInfo(w fieldWriter, item spindle.QueueItem) {
	w.field("Audio", item.PrimaryAudioDescription, w.styles.Text)
}

// renderEncodingConfig renders the encoding config line
// (encoder + preset + quality + tune).
func renderEncodingConfig(w fieldWriter, item spindle.QueueItem) {
	enc := item.Encoding
	if enc == nil || enc.Preset == "" {
		return
	}
	var parts []string

	if enc.Encoder != "" {
		parts = append(parts, enc.Encoder)
	}
	parts = append(parts, fmt.Sprintf("Preset %s", enc.Preset))
	if enc.Quality != "" {
		parts = append(parts, enc.Quality)
	}
	if enc.Tune != "" {
		parts = append(parts, fmt.Sprintf("Tune %s", enc.Tune))
	}

	w.field("Config", strings.Join(parts, " • "), w.styles.AccentText)
}

// renderContentID renders the episode identification summary.
func renderContentID(w fieldWriter, item spindle.QueueItem) {
	cid := item.ContentID
	if cid == nil || strings.TrimSpace(cid.Method) == "" {
		return
	}
	value := cid.Method
	if cid.TranscribedEpisodes > 0 || cid.MatchedEpisodes > 0 {
		value += fmt.Sprintf(" · %d matched · %d unresolved · %d low confidence",
			cid.MatchedEpisodes, cid.UnresolvedEpisodes, cid.LowConfidenceCount)
	}
	w.field("ID", value, w.styles.Text)
}

// renderEncodeStats renders duration and average speed (for completed).
func renderEncodeStats(w fieldWriter, item spindle.QueueItem) {
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

	w.field("Encode", strings.Join(parts, " @ "), w.styles.Text)
}

// renderValidationSummary renders a one-line validation summary.
func renderValidationSummary(w fieldWriter, item spindle.QueueItem) {
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
		w.field("Checks", "Passed · "+value, w.styles.SuccessText)
	} else {
		w.field("Checks", "Failed · "+value, w.styles.DangerText)
	}
}

// renderCropInfo renders the crop detection line.
func renderCropInfo(w fieldWriter, item spindle.QueueItem) {
	enc := item.Encoding
	if enc == nil {
		return
	}

	if enc.CropRequired && enc.CropFilter != "" {
		// Strip "crop=" prefix for cleaner display
		w.field("Crop", strings.TrimPrefix(enc.CropFilter, "crop="), w.styles.AccentText)
	} else if enc.CropMessage != "" {
		// Detection complete but no cropping needed
		w.field("Crop", "None", w.styles.FaintText)
	}
}

// renderSubtitleSummary renders the subtitle source summary: a count for
// multi-episode items, a plain source label for movies and single items.
func renderSubtitleSummary(w fieldWriter, item spindle.QueueItem) {
	episodes, _ := item.EpisodeSnapshot()
	if len(episodes) == 0 {
		return
	}

	count := 0
	for _, ep := range episodes {
		if strings.EqualFold(strings.TrimSpace(ep.SubtitleSource), "whisperx") {
			count++
		}
	}
	if count == 0 {
		return
	}

	if len(episodes) == 1 {
		w.field("Subs", "WhisperX", w.styles.AccentText)
		return
	}
	w.field("Subs", fmt.Sprintf("%d WhisperX", count), w.styles.AccentText)
}
