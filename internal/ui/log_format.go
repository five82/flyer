package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

func formatLogEvents(events []spindle.LogEvent) []string {
	if len(events) == 0 {
		return nil
	}
	lines := make([]string, 0, len(events))
	for _, evt := range events {
		lines = append(lines, formatLogEvent(evt))
	}
	return lines
}

func formatLogEvent(evt spindle.LogEvent) string {
	ts := evt.Timestamp
	if parsed := evt.ParsedTime(); !parsed.IsZero() {
		ts = parsed.In(time.Local).Format("2006-01-02 15:04:05")
	}
	level := strings.ToUpper(strings.TrimSpace(evt.Level))
	if level == "" {
		level = "INFO"
	}
	parts := []string{ts, level}
	if component := strings.TrimSpace(evt.Component); component != "" {
		parts = append(parts, fmt.Sprintf("[%s]", component))
	}
	subject := composeSubject(evt.ItemID, evt.Stage)
	header := strings.Join(parts, " ")
	if subject != "" {
		header += " " + subject
	}
	message := strings.TrimSpace(evt.Message)
	if message != "" {
		header += " – " + message
	}
	if len(evt.Details) == 0 {
		return header
	}
	var builder strings.Builder
	builder.WriteString(header)
	for _, detail := range evt.Details {
		label := strings.TrimSpace(detail.Label)
		value := strings.TrimSpace(detail.Value)
		if label == "" || value == "" {
			continue
		}
		builder.WriteString("\n    - ")
		builder.WriteString(label)
		builder.WriteString(": ")
		builder.WriteString(value)
	}
	return builder.String()
}

func composeSubject(itemID int64, stage string) string {
	stage = strings.TrimSpace(stage)
	switch {
	case itemID > 0 && stage != "":
		return fmt.Sprintf("Item #%d (%s)", itemID, stage)
	case itemID > 0:
		return fmt.Sprintf("Item #%d", itemID)
	default:
		return stage
	}
}
