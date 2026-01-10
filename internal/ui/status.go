package ui

import (
	"strings"

	"github.com/five82/flyer/internal/spindle"
)

// statusPriority defines the display order for queue items.
// Lower values appear first (higher priority).
var statusPriority = map[string]int{
	"failed":              0,
	"review":              1,
	"subtitling":          2,
	"encoding":            3,
	"organizing":          4,
	"ripping":             5,
	"episode_identifying": 6,
	"identifying":         7,
	"episode_identified":  8,
	"ripped":              9,
	"subtitled":           10,
	"encoded":             11,
	"identified":          12,
	"pending":             13,
	"completed":           14,
}

// statusRank returns the display priority for a status.
func statusRank(status string) int {
	if rank, ok := statusPriority[strings.ToLower(strings.TrimSpace(status))]; ok {
		return rank
	}
	return 999
}

// effectiveQueueStage determines the effective stage for a queue item,
// preferring progress.stage over status for active items.
func effectiveQueueStage(item spindle.QueueItem) string {
	status := strings.ToLower(strings.TrimSpace(item.Status))
	switch status {
	case "completed":
		return "completed"
	case "failed", "review":
		// Terminal / attention states should override any stale progress.stage.
		return status
	}

	if stage := strings.ToLower(strings.TrimSpace(item.Progress.Stage)); stage != "" {
		return stage
	}
	return status
}

// normalizeEpisodeStage normalizes a status/stage string to a canonical form
// for the pipeline display.
func normalizeEpisodeStage(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return ""
	}
	// Identification stages (disc or episode)
	if s == "episode_identifying" || strings.Contains(s, "episode identification") {
		return "identifying"
	}
	if s == "episode_identified" || strings.Contains(s, "episode identified") {
		return "identified"
	}
	if s == "identifying" || s == "identified" {
		return s
	}
	// Subtitling stages
	if s == "subtitled" {
		return "subtitled"
	}
	if strings.HasPrefix(s, "subtitl") {
		return "subtitling"
	}
	// Encoding stages
	if s == "encoded" {
		return "encoded"
	}
	if strings.HasPrefix(s, "encod") {
		return "encoding"
	}
	// Ripping stages
	if s == "ripped" {
		return "ripped"
	}
	if strings.HasPrefix(s, "rip") {
		return "ripping"
	}
	// Organizing stage (distinct from final)
	if s == "organizing" {
		return "organizing"
	}
	// Final/completed states
	if s == "final" || s == "completed" || s == "complete" || s == "success" || s == "done" {
		return "final"
	}
	return "planned"
}

// pipelineStageForStatus maps a normalized stage to a pipeline stage ID.
func pipelineStageForStatus(stage string) string {
	switch stage {
	case "identifying", "identified":
		return "identifying"
	case "ripping", "ripped":
		return "ripped"
	case "encoding", "encoded":
		return "encoded"
	case "subtitling", "subtitled":
		return "subtitled"
	case "organizing":
		return "organizing"
	case "final", "completed":
		return "final"
	case "planned", "pending":
		return "planned"
	}
	return "planned"
}

// singleItemPipelineCount determines the count for a single-item pipeline stage
// based on file evidence and current stage.
func singleItemPipelineCount(stageID string, item spindle.QueueItem, activeStage string, plannedCount int) int {
	// For a single item, "planned" is always known once it's in the queue.
	if stageID == "planned" {
		return plannedCount
	}

	// Prefer concrete file evidence, then fall back to inferred stage.
	switch stageID {
	case "identifying":
		// Identification is complete once we've moved past it.
		switch activeStage {
		case "identified", "ripping", "ripped", "encoding", "encoded", "subtitling", "subtitled", "organizing", "final":
			return plannedCount
		}
	case "ripped":
		if strings.TrimSpace(item.RippedFile) != "" {
			return plannedCount
		}
		// Once we've moved past ripping, treat ripped as done.
		switch activeStage {
		case "ripped", "encoding", "encoded", "subtitling", "subtitled", "organizing", "final":
			return plannedCount
		}
	case "encoded":
		if strings.TrimSpace(item.EncodedFile) != "" {
			return plannedCount
		}
		switch activeStage {
		case "encoded", "subtitling", "subtitled", "organizing", "final":
			return plannedCount
		}
	case "subtitled":
		switch activeStage {
		case "subtitled", "organizing", "final":
			return plannedCount
		}
	case "organizing":
		// Organizing is complete only when we've reached final.
		if activeStage == "final" {
			return plannedCount
		}
	case "final":
		if strings.TrimSpace(item.FinalFile) != "" {
			return plannedCount
		}
		if activeStage == "final" {
			return plannedCount
		}
	}

	return 0
}
