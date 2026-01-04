package spindle

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// StatusFetcher defines the interface for fetching Spindle status and logs.
// This interface is implemented by *Client and can be used for testing.
type StatusFetcher interface {
	FetchStatus(ctx context.Context) (*StatusResponse, error)
	FetchQueue(ctx context.Context) ([]QueueItem, error)
	FetchLogs(ctx context.Context, query LogQuery) (LogBatch, error)
	FetchLogTail(ctx context.Context, query LogTailQuery) (LogTailBatch, error)
}

// Ensure Client implements StatusFetcher at compile time.
var _ StatusFetcher = (*Client)(nil)

// Client talks to the Spindle HTTP API.
type Client struct {
	baseURL   *url.URL
	http      *http.Client
	userAgent string
}

const (
	defaultAPIBind   = "127.0.0.1:7487"
	defaultUserAgent = "flyer/0.1"
	requestTimeout   = 5 * time.Second
)

// NewClient builds a Client using the provided apiBind host:port value.
func NewClient(apiBind string) (*Client, error) {
	base, err := parseBaseURL(apiBind)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: base,
		http: &http.Client{
			Timeout: requestTimeout,
		},
		userAgent: defaultUserAgent,
	}, nil
}

// FetchStatus retrieves daemon and workflow status information.
func (c *Client) FetchStatus(ctx context.Context) (*StatusResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}
	var payload StatusResponse
	if err := c.do(ctx, http.MethodGet, "/api/status", &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// FetchQueue retrieves the current queue snapshot.
func (c *Client) FetchQueue(ctx context.Context) ([]QueueItem, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}
	var payload QueueListResponse
	if err := c.do(ctx, http.MethodGet, "/api/queue", &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

// LogQuery configures /api/logs requests.
type LogQuery struct {
	Since     uint64
	Limit     int
	Follow    bool
	Tail      bool
	ItemID    int64
	Level     string
	Component string
	Lane      string
	Request   string
}

// FetchLogs retrieves log events using the daemon's streaming API.
func (c *Client) FetchLogs(ctx context.Context, query LogQuery) (LogBatch, error) {
	if c == nil {
		return LogBatch{}, fmt.Errorf("client is nil")
	}
	values := url.Values{}
	if query.Since > 0 {
		values.Set("since", strconv.FormatUint(query.Since, 10))
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Follow {
		values.Set("follow", "1")
	}
	if query.Tail {
		values.Set("tail", "1")
	}
	if query.ItemID > 0 {
		values.Set("item", strconv.FormatInt(query.ItemID, 10))
	}
	if level := strings.TrimSpace(query.Level); level != "" {
		values.Set("level", level)
	}
	if component := strings.TrimSpace(query.Component); component != "" {
		values.Set("component", component)
	}
	if lane := strings.TrimSpace(query.Lane); lane != "" {
		values.Set("lane", lane)
	}
	if req := strings.TrimSpace(query.Request); req != "" {
		values.Set("request", req)
	}
	rel := &url.URL{Path: "/api/logs", RawQuery: values.Encode()}
	var payload LogBatch
	if err := c.doURL(ctx, http.MethodGet, rel, &payload); err != nil {
		return LogBatch{}, err
	}
	return payload, nil
}

// LogTailQuery configures /api/logtail requests.
type LogTailQuery struct {
	ItemID int64
	Offset int64
	Limit  int
	Follow bool
	WaitMS int
}

// FetchLogTail retrieves raw log lines for an item's log file.
func (c *Client) FetchLogTail(ctx context.Context, query LogTailQuery) (LogTailBatch, error) {
	if c == nil {
		return LogTailBatch{}, fmt.Errorf("client is nil")
	}
	if query.ItemID <= 0 {
		return LogTailBatch{}, fmt.Errorf("item id required")
	}
	values := url.Values{}
	values.Set("item", strconv.FormatInt(query.ItemID, 10))
	values.Set("offset", strconv.FormatInt(query.Offset, 10))
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Follow {
		values.Set("follow", "1")
	}
	if query.WaitMS > 0 {
		values.Set("wait_ms", strconv.Itoa(query.WaitMS))
	}
	rel := &url.URL{Path: "/api/logtail", RawQuery: values.Encode()}
	var payload LogTailBatch
	if err := c.doURL(ctx, http.MethodGet, rel, &payload); err != nil {
		return LogTailBatch{}, err
	}
	return payload, nil
}

func (c *Client) do(ctx context.Context, method, path string, dest any) error {
	rel := &url.URL{Path: path}
	return c.doURL(ctx, method, rel, dest)
}

func (c *Client) doURL(ctx context.Context, method string, rel *url.URL, dest any) error {
	reqURL := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("api %s returned status %d", rel.String(), resp.StatusCode)
	}
	if dest == nil {
		return nil
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func parseBaseURL(apiBind string) (*url.URL, error) {
	trimmed := strings.TrimSpace(apiBind)
	if trimmed == "" {
		trimmed = defaultAPIBind
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse api_bind %q: %w", apiBind, err)
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}
