package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

// metadataRow represents a single metadata key-value pair.
type metadataRow struct {
	key   string
	value string
}

// checkMatch checks if two file paths refer to the same file.
// Compares exact paths, case-insensitive paths, and basenames.
func checkMatch(target, candidate string) bool {
	target = strings.TrimSpace(target)
	candidate = strings.TrimSpace(candidate)
	if target == "" || candidate == "" {
		return false
	}
	if strings.EqualFold(target, candidate) {
		return true
	}
	// Check if either is a suffix of the other (handles relative vs absolute paths)
	targetLower := strings.ToLower(target)
	candidateLower := strings.ToLower(candidate)
	if strings.HasSuffix(candidateLower, targetLower) || strings.HasSuffix(targetLower, candidateLower) {
		return true
	}
	return false
}

// buildTitleLookups creates lookup maps from a RipSpec summary for efficient title info access.
func buildTitleLookups(summary spindle.RipSpecSummary) (map[int]*spindle.RipSpecTitleInfo, map[string]int) {
	titleLookup := make(map[int]*spindle.RipSpecTitleInfo, len(summary.Titles))
	for i := range summary.Titles {
		t := summary.Titles[i]
		titleLookup[t.ID] = &t
	}

	episodeTitleIndex := make(map[string]int, len(summary.Episodes))
	for _, ep := range summary.Episodes {
		if ep.TitleID > 0 {
			if key := strings.ToLower(strings.TrimSpace(ep.Key)); key != "" {
				episodeTitleIndex[key] = ep.TitleID
			}
		}
	}
	return titleLookup, episodeTitleIndex
}

// parseTimestamp parses an RFC3339 timestamp string.
func parseTimestamp(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// formatTimestamp formats a timestamp for display.
func formatTimestamp(t time.Time, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	local := t.In(time.Local)
	if local.Year() == now.Year() && local.YearDay() == now.YearDay() {
		return local.Format("15:04:05")
	}
	return local.Format("Jan 02 15:04")
}

// humanizeDuration formats a duration as relative time (e.g., "5m ago").
func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// humanizeDurationLong formats duration as "Xh Ym" (matching tview's format).
func humanizeDurationLong(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// formatDuration formats a duration with hours, minutes, and seconds.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// formatBytes formats bytes as human-readable size (GiB/MiB).
func formatBytes(bytes int64) string {
	const (
		gib = 1024 * 1024 * 1024
		mib = 1024 * 1024
	)
	if bytes >= gib {
		return fmt.Sprintf("%.2f GiB", float64(bytes)/gib)
	}
	return fmt.Sprintf("%.2f MiB", float64(bytes)/mib)
}

// clampPercent ensures percent is between 0 and 100.
func clampPercent(percent float64) float64 {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

// formatRuntime formats seconds as "Xm".
func formatRuntime(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	return fmt.Sprintf("%dm", seconds/60)
}

// detectMediaType extracts the media type from metadata JSON.
func detectMediaType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"media_type", "type"} {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok {
				return strings.ToLower(strings.TrimSpace(s))
			}
		}
	}
	return ""
}

// summarizeMetadata extracts displayable metadata rows from JSON.
func summarizeMetadata(raw json.RawMessage) []metadataRow {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj) == 0 {
		return nil
	}
	mediaType := ""
	if mt, ok := obj["media_type"]; ok {
		if s, ok := mt.(string); ok {
			mediaType = strings.ToLower(strings.TrimSpace(s))
		}
	}
	skip := map[string]struct{}{
		"vote_average": {},
		"vote_count":   {},
		"overview":     {},
	}
	var rows []metadataRow
	for k, val := range obj {
		lk := strings.ToLower(strings.TrimSpace(k))
		if _, ignore := skip[lk]; ignore {
			continue
		}
		if mediaType == "movie" && lk == "movie" {
			continue
		}
		if mediaType == "tv" && lk == "tv" {
			continue
		}
		if mediaType == "movie" && strings.EqualFold(k, "season_number") {
			continue
		}
		switch v := val.(type) {
		case string:
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			rows = append(rows, metadataRow{key: k, value: v})
		case float64:
			rows = append(rows, metadataRow{key: k, value: fmt.Sprintf("%g", v)})
		case bool:
			rows = append(rows, metadataRow{key: k, value: fmt.Sprintf("%t", v)})
		}
	}
	return rows
}

// prettifyMetaKey formats a metadata key for display.
func prettifyMetaKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, "_", " ")
	key = strings.ReplaceAll(key, ".", " ")
	parts := strings.Fields(key)
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}
