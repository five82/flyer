package spindle

import (
	"encoding/json"
	"sort"
	"strings"
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
	ID                      int64                     `json:"id"`
	DiscTitle               string                    `json:"discTitle"`
	SourcePath              string                    `json:"sourcePath"`
	Status                  string                    `json:"status"`
	ProcessingLane          string                    `json:"processingLane"`
	DraptoPreset            string                    `json:"draptoPresetProfile,omitempty"`
	Progress                QueueProgress             `json:"progress"`
	Encoding                *EncodingStatus           `json:"encoding,omitempty"`
	ErrorMessage            string                    `json:"errorMessage"`
	CreatedAt               string                    `json:"createdAt"`
	UpdatedAt               string                    `json:"updatedAt"`
	DiscFingerprint         string                    `json:"discFingerprint"`
	RippedFile              string                    `json:"rippedFile"`
	EncodedFile             string                    `json:"encodedFile"`
	FinalFile               string                    `json:"finalFile"`
	ItemLogPath             string                    `json:"itemLogPath"`
	NeedsReview             bool                      `json:"needsReview"`
	ReviewReason            string                    `json:"reviewReason"`
	Metadata                json.RawMessage           `json:"metadata"`
	RipSpec                 json.RawMessage           `json:"ripSpec"`
	Episodes                []EpisodeStatus           `json:"episodes"`
	EpisodeTotals           *EpisodeTotals            `json:"episodeTotals"`
	EpisodesSynced          bool                      `json:"episodesSynchronized"`
	SubtitleGeneration      *SubtitleGenerationStatus `json:"subtitleGeneration"`
	PrimaryAudioDescription string                    `json:"primaryAudioDescription"`
	CommentaryCount         int                       `json:"commentaryCount"`
}

// QueueProgress tracks stage progress for an item.
type QueueProgress struct {
	Stage   string  `json:"stage"`
	Percent float64 `json:"percent"`
	Message string  `json:"message"`
}

type EncodingStatus struct {
	JobLabel     string              `json:"jobLabel,omitempty"`
	EpisodeKey   string              `json:"episodeKey,omitempty"`
	EpisodeIndex int                 `json:"episodeIndex,omitempty"`
	EpisodeCount int                 `json:"episodeCount,omitempty"`
	Stage        string              `json:"stage,omitempty"`
	Message      string              `json:"message,omitempty"`
	Percent      float64             `json:"percent,omitempty"`
	ETASeconds   float64             `json:"etaSeconds,omitempty"`
	Speed        float64             `json:"speed,omitempty"`
	FPS          float64             `json:"fps,omitempty"`
	Bitrate      string              `json:"bitrate,omitempty"`
	TotalFrames  int64               `json:"totalFrames,omitempty"`
	CurrentFrame int64               `json:"currentFrame,omitempty"`
	Hardware     *EncodingHardware   `json:"hardware,omitempty"`
	Video        *EncodingVideo      `json:"video,omitempty"`
	Crop         *EncodingCrop       `json:"crop,omitempty"`
	Config       *EncodingConfig     `json:"config,omitempty"`
	Validation   *EncodingValidation `json:"validation,omitempty"`
	Warning      string              `json:"warning,omitempty"`
	Error        *EncodingIssue      `json:"error,omitempty"`
	Result       *EncodingResult     `json:"result,omitempty"`
}

type EncodingHardware struct {
	Hostname string `json:"hostname,omitempty"`
}

type EncodingVideo struct {
	InputFile        string `json:"inputFile,omitempty"`
	OutputFile       string `json:"outputFile,omitempty"`
	Duration         string `json:"duration,omitempty"`
	Resolution       string `json:"resolution,omitempty"`
	Category         string `json:"category,omitempty"`
	DynamicRange     string `json:"dynamicRange,omitempty"`
	AudioDescription string `json:"audioDescription,omitempty"`
}

type EncodingCrop struct {
	Message  string `json:"message,omitempty"`
	Crop     string `json:"crop,omitempty"`
	Required bool   `json:"required,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type EncodingConfig struct {
	Encoder            string                  `json:"encoder,omitempty"`
	Preset             string                  `json:"preset,omitempty"`
	Tune               string                  `json:"tune,omitempty"`
	Quality            string                  `json:"quality,omitempty"`
	PixelFormat        string                  `json:"pixelFormat,omitempty"`
	MatrixCoefficients string                  `json:"matrixCoefficients,omitempty"`
	AudioCodec         string                  `json:"audioCodec,omitempty"`
	AudioDescription   string                  `json:"audioDescription,omitempty"`
	DraptoPreset       string                  `json:"draptoPreset,omitempty"`
	PresetSettings     []EncodingPresetSetting `json:"presetSettings,omitempty"`
	SVTParams          string                  `json:"svtParams,omitempty"`
}

type EncodingPresetSetting struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type EncodingValidation struct {
	Passed bool                     `json:"passed,omitempty"`
	Steps  []EncodingValidationStep `json:"steps,omitempty"`
}

type EncodingValidationStep struct {
	Name    string `json:"name,omitempty"`
	Passed  bool   `json:"passed,omitempty"`
	Details string `json:"details,omitempty"`
}

type EncodingIssue struct {
	Title      string `json:"title,omitempty"`
	Message    string `json:"message,omitempty"`
	Context    string `json:"context,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

type EncodingResult struct {
	InputFile            string  `json:"inputFile,omitempty"`
	OutputFile           string  `json:"outputFile,omitempty"`
	OutputPath           string  `json:"outputPath,omitempty"`
	OriginalSize         int64   `json:"originalSize,omitempty"`
	EncodedSize          int64   `json:"encodedSize,omitempty"`
	VideoStream          string  `json:"videoStream,omitempty"`
	AudioStream          string  `json:"audioStream,omitempty"`
	AverageSpeed         float64 `json:"averageSpeed,omitempty"`
	DurationSeconds      float64 `json:"durationSeconds,omitempty"`
	SizeReductionPercent float64 `json:"sizeReductionPercent,omitempty"`
}

// ETADuration returns the ETA as a duration when available.
func (e *EncodingStatus) ETADuration() time.Duration {
	if e == nil || e.ETASeconds <= 0 {
		return 0
	}
	return time.Duration(e.ETASeconds * float64(time.Second))
}

// FramePercent returns the current frame progress [0,1].
func (e *EncodingStatus) FramePercent() float64 {
	if e == nil || e.TotalFrames <= 0 || e.CurrentFrame <= 0 {
		return 0
	}
	return float64(e.CurrentFrame) / float64(e.TotalFrames)
}

type EpisodeStatus struct {
	Key                       string         `json:"key"`
	Season                    int            `json:"season"`
	Episode                   int            `json:"episode"`
	Title                     string         `json:"title"`
	Stage                     string         `json:"stage"`
	Status                    string         `json:"status,omitempty"`       // pending, completed, failed
	ErrorMessage              string         `json:"errorMessage,omitempty"` // per-episode error
	Active                    bool           `json:"active,omitempty"`
	Progress                  *QueueProgress `json:"progress,omitempty"`
	RuntimeSeconds            int            `json:"runtimeSeconds"`
	SourceTitleID             int            `json:"sourceTitleId"`
	SourceTitle               string         `json:"sourceTitle"`
	OutputBasename            string         `json:"outputBasename"`
	RippedPath                string         `json:"rippedPath"`
	EncodedPath               string         `json:"encodedPath"`
	FinalPath                 string         `json:"finalPath"`
	SubtitleSource            string         `json:"subtitleSource"`
	SubtitleLanguage          string         `json:"subtitleLanguage"`
	GeneratedSubtitleSource   string         `json:"generatedSubtitleSource"`
	GeneratedSubtitleLanguage string         `json:"generatedSubtitleLanguage"`
	GeneratedSubtitleDecision string         `json:"generatedSubtitleDecision"`
	MatchScore                float64        `json:"matchScore"`
	MatchedEpisode            int            `json:"matchedEpisode"`
}

// IsFailed returns true if the episode has a failed status.
func (e EpisodeStatus) IsFailed() bool {
	return strings.EqualFold(strings.TrimSpace(e.Status), "failed")
}

// FilterFailed returns episodes with failed status from the given slice.
func FilterFailed(episodes []EpisodeStatus) []EpisodeStatus {
	var failed []EpisodeStatus
	for _, ep := range episodes {
		if ep.IsFailed() {
			failed = append(failed, ep)
		}
	}
	return failed
}

type EpisodeTotals struct {
	Planned int `json:"planned"`
	Ripped  int `json:"ripped"`
	Encoded int `json:"encoded"`
	Final   int `json:"final"`
}

type SubtitleGenerationStatus struct {
	OpenSubtitles         int  `json:"opensubtitles"`
	WhisperX              int  `json:"whisperx"`
	ExpectedOpenSubtitles bool `json:"expectedOpenSubtitles"`
	FallbackUsed          bool `json:"fallbackUsed"`
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

type LogTailBatch struct {
	Lines  []string `json:"lines"`
	Offset int64    `json:"offset"`
}

// RipSpecSummary describes the subset of rip spec details Flyer cares about.
type RipSpecSummary struct {
	ContentKey string               `json:"content_key"`
	Metadata   map[string]any       `json:"metadata"`
	Titles     []RipSpecTitleInfo   `json:"titles"`
	Episodes   []RipSpecEpisodeInfo `json:"episodes"`
}

// RipSpecTitleInfo captures per-title fingerprint information.
type RipSpecTitleInfo struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Duration           int    `json:"duration"`
	Playlist           string `json:"playlist"`
	SegmentCount       int    `json:"segment_count"`
	SegmentMap         string `json:"segment_map"`
	Season             int    `json:"season"`
	Episode            int    `json:"episode"`
	EpisodeTitle       string `json:"episode_title"`
	EpisodeAirDate     string `json:"episode_air_date"`
	ContentFingerprint string `json:"content_fingerprint"`
}

// RipSpecEpisodeInfo links rip spec episodes to playlist metadata for lookups.
type RipSpecEpisodeInfo struct {
	Key            string `json:"key"`
	TitleID        int    `json:"title_id"`
	Season         int    `json:"season"`
	Episode        int    `json:"episode"`
	EpisodeTitle   string `json:"episode_title"`
	OutputBasename string `json:"output_basename"`
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

// DraptoPresetLabel formats the Drapto preset profile for display.
func (q QueueItem) DraptoPresetLabel() string {
	value := strings.TrimSpace(q.DraptoPreset)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	switch lower {
	case "default":
		return "Default"
	case "clean":
		return "Clean"
	case "grain":
		return "Grain"
	default:
		return strings.ToUpper(lower[:1]) + lower[1:]
	}
}

// EpisodeSnapshot normalizes per-episode data for the UI, deriving it from the
// raw rip spec when the API fields are unavailable.
func (q QueueItem) EpisodeSnapshot() ([]EpisodeStatus, EpisodeTotals) {
	if len(q.Episodes) > 0 {
		copyEpisodes := make([]EpisodeStatus, len(q.Episodes))
		copy(copyEpisodes, q.Episodes)
		if q.EpisodeTotals != nil {
			return copyEpisodes, *q.EpisodeTotals
		}
		return copyEpisodes, tallyEpisodeTotals(copyEpisodes)
	}
	if len(q.RipSpec) == 0 {
		return nil, EpisodeTotals{}
	}
	return deriveEpisodesFromRipSpec(q.RipSpec)
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

func tallyEpisodeTotals(list []EpisodeStatus) EpisodeTotals {
	var totals EpisodeTotals
	for range list {
		totals.Planned++
	}
	for _, ep := range list {
		if strings.TrimSpace(ep.RippedPath) != "" {
			totals.Ripped++
		}
		if strings.TrimSpace(ep.EncodedPath) != "" {
			totals.Encoded++
		}
		if strings.TrimSpace(ep.FinalPath) != "" {
			totals.Final++
		}
	}
	return totals
}

func deriveEpisodesFromRipSpec(raw json.RawMessage) ([]EpisodeStatus, EpisodeTotals) {
	var env ripSpecEnvelope
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Episodes) == 0 {
		return nil, EpisodeTotals{}
	}
	lookup := env.assetsByKey()
	titles := env.titleByID()
	statuses := make([]EpisodeStatus, 0, len(env.Episodes))
	for _, ep := range env.Episodes {
		status := EpisodeStatus{
			Key:            strings.ToLower(strings.TrimSpace(ep.Key)),
			Season:         ep.Season,
			Episode:        ep.Episode,
			Title:          strings.TrimSpace(ep.EpisodeTitle),
			Stage:          "planned",
			RuntimeSeconds: ep.RuntimeSeconds,
			SourceTitleID:  ep.TitleID,
			OutputBasename: strings.TrimSpace(ep.OutputBasename),
		}
		if title, ok := titles[ep.TitleID]; ok {
			if status.Title == "" {
				status.Title = strings.TrimSpace(title.EpisodeTitle)
			}
			if status.Title == "" {
				status.Title = strings.TrimSpace(title.Name)
			}
			status.SourceTitle = strings.TrimSpace(title.Name)
			if status.RuntimeSeconds == 0 {
				status.RuntimeSeconds = title.Duration
			}
		}
		if asset, ok := lookup[status.Key]; ok {
			if asset.Ripped != "" {
				status.RippedPath = asset.Ripped
				status.Stage = "ripped"
			}
			if asset.Encoded != "" {
				status.EncodedPath = asset.Encoded
				status.Stage = "encoded"
			}
			if asset.Subtitled != "" {
				status.Stage = "subtitled"
			}
			if asset.Final != "" {
				status.FinalPath = asset.Final
				status.Stage = "final"
			}
			// Per-episode status and error
			if asset.Status != "" {
				status.Status = asset.Status
			}
			if asset.ErrorMessage != "" {
				status.ErrorMessage = asset.ErrorMessage
			}
		}
		statuses = append(statuses, status)
	}
	sort.SliceStable(statuses, func(i, j int) bool {
		if statuses[i].Season != statuses[j].Season {
			return statuses[i].Season < statuses[j].Season
		}
		if statuses[i].Episode != statuses[j].Episode {
			return statuses[i].Episode < statuses[j].Episode
		}
		return statuses[i].Key < statuses[j].Key
	})
	return statuses, tallyEpisodeTotals(statuses)
}

type ripSpecEnvelope struct {
	Titles   []ripSpecTitle   `json:"titles"`
	Episodes []ripSpecEpisode `json:"episodes"`
	Assets   ripSpecAssets    `json:"assets"`
}

type ripSpecTitle struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	EpisodeTitle string `json:"episode_title"`
	Duration     int    `json:"duration"`
}

type ripSpecEpisode struct {
	Key            string `json:"key"`
	TitleID        int    `json:"title_id"`
	Season         int    `json:"season"`
	Episode        int    `json:"episode"`
	EpisodeTitle   string `json:"episode_title"`
	RuntimeSeconds int    `json:"runtime_seconds"`
	OutputBasename string `json:"output_basename"`
}

type ripSpecAssets struct {
	Ripped    []ripSpecAsset `json:"ripped"`
	Encoded   []ripSpecAsset `json:"encoded"`
	Subtitled []ripSpecAsset `json:"subtitled"`
	Final     []ripSpecAsset `json:"final"`
}

type ripSpecAsset struct {
	EpisodeKey string `json:"episode_key"`
	Path       string `json:"path"`
	Status     string `json:"status,omitempty"`
	ErrorMsg   string `json:"error_msg,omitempty"`
}

type assetPaths struct {
	Ripped       string
	Encoded      string
	Subtitled    string
	Final        string
	Status       string // Overall status (failed if any asset failed)
	ErrorMessage string // Error message from failed asset
}

func (e ripSpecEnvelope) titleByID() map[int]ripSpecTitle {
	if len(e.Titles) == 0 {
		return nil
	}
	lookup := make(map[int]ripSpecTitle, len(e.Titles))
	for _, title := range e.Titles {
		lookup[title.ID] = title
	}
	return lookup
}

func (e ripSpecEnvelope) assetsByKey() map[string]assetPaths {
	lookup := make(map[string]assetPaths)

	normalizeKey := func(key string) string {
		return strings.ToLower(strings.TrimSpace(key))
	}

	for _, asset := range e.Assets.Ripped {
		key := normalizeKey(asset.EpisodeKey)
		if key == "" {
			continue
		}
		entry := lookup[key]
		entry.Ripped = asset.Path
		lookup[key] = entry
	}

	for _, asset := range e.Assets.Encoded {
		key := normalizeKey(asset.EpisodeKey)
		if key == "" {
			continue
		}
		entry := lookup[key]
		entry.Encoded = asset.Path
		if asset.Status != "" {
			entry.Status = asset.Status
		}
		if asset.ErrorMsg != "" {
			entry.ErrorMessage = asset.ErrorMsg
		}
		lookup[key] = entry
	}

	for _, asset := range e.Assets.Subtitled {
		key := normalizeKey(asset.EpisodeKey)
		if key == "" {
			continue
		}
		entry := lookup[key]
		entry.Subtitled = asset.Path
		if strings.EqualFold(asset.Status, "failed") {
			entry.Status = asset.Status
			entry.ErrorMessage = asset.ErrorMsg
		}
		lookup[key] = entry
	}

	for _, asset := range e.Assets.Final {
		key := normalizeKey(asset.EpisodeKey)
		if key == "" {
			continue
		}
		entry := lookup[key]
		entry.Final = asset.Path
		if strings.EqualFold(asset.Status, "failed") {
			entry.Status = asset.Status
			entry.ErrorMessage = asset.ErrorMsg
		}
		lookup[key] = entry
	}

	return lookup
}
