package spindle

import (
	"encoding/json"
	"time"
)

const spindleTimestampLayout = "2006-01-02 15:04:05"

// StatusResponse mirrors the payload returned by /api/status.
type StatusResponse struct {
	Running      bool               `json:"running"`
	PID          int                `json:"pid"`
	QueueDBPath  string             `json:"queueDbPath"`
	LockFilePath string             `json:"lockFilePath"`
	Workflow     WorkflowStatus     `json:"workflow"`
	Dependencies []DependencyStatus `json:"dependencies"`
}

// WorkflowStatus aggregates queue stats and recent activity.
type WorkflowStatus struct {
	Running     bool           `json:"running"`
	QueueStats  map[string]int `json:"queueStats"`
	LastError   string         `json:"lastError"`
	LastItem    *QueueItem     `json:"lastItem"`
	StageHealth []StageHealth  `json:"stageHealth"`
}

// StageHealth reflects readiness indicators for workflow components.
type StageHealth struct {
	Name   string `json:"name"`
	Ready  bool   `json:"ready"`
	Detail string `json:"detail"`
}

// DependencyStatus reports an external dependency health check.
type DependencyStatus struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description"`
	Optional    bool   `json:"optional"`
	Available   bool   `json:"available"`
	Detail      string `json:"detail"`
}

// QueueListResponse mirrors /api/queue.
type QueueListResponse struct {
	Items []QueueItem `json:"items"`
}

// QueueItem describes a queue entry in transport-friendly form.
type QueueItem struct {
	ID                int64           `json:"id"`
	DiscTitle         string          `json:"discTitle"`
	SourcePath        string          `json:"sourcePath"`
	Status            string          `json:"status"`
	ProcessingLane    string          `json:"processingLane"`
	Progress          QueueProgress   `json:"progress"`
	ErrorMessage      string          `json:"errorMessage"`
	CreatedAt         string          `json:"createdAt"`
	UpdatedAt         string          `json:"updatedAt"`
	DiscFingerprint   string          `json:"discFingerprint"`
	RippedFile        string          `json:"rippedFile"`
	EncodedFile       string          `json:"encodedFile"`
	FinalFile         string          `json:"finalFile"`
	BackgroundLogPath string          `json:"backgroundLogPath"`
	NeedsReview       bool            `json:"needsReview"`
	ReviewReason      string          `json:"reviewReason"`
	Metadata          json.RawMessage `json:"metadata"`
	RipSpec           json.RawMessage `json:"ripSpec"`
}

// QueueProgress tracks stage progress for an item.
type QueueProgress struct {
	Stage   string  `json:"stage"`
	Percent float64 `json:"percent"`
	Message string  `json:"message"`
}

// LogEvent represents a single log entry from /api/logs.
type LogEvent struct {
	Sequence  uint64            `json:"seq"`
	Timestamp string            `json:"ts"`
	Level     string            `json:"level"`
	Message   string            `json:"msg"`
	Component string            `json:"component"`
	Stage     string            `json:"stage"`
	ItemID    int64             `json:"item_id"`
	Fields    map[string]string `json:"fields"`
	Details   []DetailField     `json:"details"`
}

// ParsedTime returns the timestamp as time.Time when possible.
func (e LogEvent) ParsedTime() time.Time {
	return parseTime(e.Timestamp)
}

// DetailField mirrors the API detail payloads.
type DetailField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// LogBatch aggregates a slice of log events with the next sequence cursor.
type LogBatch struct {
	Events []LogEvent `json:"events"`
	Next   uint64     `json:"next"`
}

// RipSpecSummary describes the subset of rip spec details Flyer cares about.
type RipSpecSummary struct {
	ContentKey string             `json:"content_key"`
	Metadata   map[string]any     `json:"metadata"`
	Titles     []RipSpecTitleInfo `json:"titles"`
}

// RipSpecTitleInfo captures per-title fingerprint information.
type RipSpecTitleInfo struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Duration           int    `json:"duration"`
	ContentFingerprint string `json:"content_fingerprint"`
}

// ParseRipSpec decodes the rip specification payload if present.
func (q QueueItem) ParseRipSpec() (RipSpecSummary, error) {
	if len(q.RipSpec) == 0 {
		return RipSpecSummary{}, nil
	}
	var summary RipSpecSummary
	if err := json.Unmarshal(q.RipSpec, &summary); err != nil {
		return RipSpecSummary{}, err
	}
	return summary, nil
}

// ParsedCreatedAt returns the parsed CreatedAt timestamp.
func (q QueueItem) ParsedCreatedAt() time.Time {
	return parseTime(q.CreatedAt)
}

// ParsedUpdatedAt returns the parsed UpdatedAt timestamp.
func (q QueueItem) ParsedUpdatedAt() time.Time {
	return parseTime(q.UpdatedAt)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	if t, err := time.ParseInLocation(spindleTimestampLayout, value, time.Local); err == nil {
		return t
	}
	return time.Time{}
}
