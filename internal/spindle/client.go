package spindle

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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

func (c *Client) do(ctx context.Context, method, path string, dest any) error {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: path})
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
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("api %s returned status %d", path, resp.StatusCode)
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
