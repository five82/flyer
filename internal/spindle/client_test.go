package spindle

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseBaseURL_DefaultsAndNormalizes(t *testing.T) {
	u, err := parseBaseURL("")
	if err != nil {
		t.Fatalf("parseBaseURL returned error: %v", err)
	}
	if u.Scheme != "http" {
		t.Fatalf("scheme = %q, want http", u.Scheme)
	}
	if u.Host != defaultAPIBind {
		t.Fatalf("host = %q, want %q", u.Host, defaultAPIBind)
	}

	u, err = parseBaseURL("http://example.com:1234/path?x=1#frag")
	if err != nil {
		t.Fatalf("parseBaseURL returned error: %v", err)
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		t.Fatalf("url not normalized: %q", u.String())
	}
}

func TestClient_FetchesEndpointsAndEncodesQueries(t *testing.T) {
	t.Parallel()

	var gotLogsQuery url.Values
	var gotLogTailQuery url.Values
	var gotUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/status":
			_ = json.NewEncoder(w).Encode(StatusResponse{Running: true, PID: 123})
		case "/api/queue":
			_ = json.NewEncoder(w).Encode(QueueListResponse{Items: []QueueItem{{ID: 42, DiscTitle: "Disc"}}})
		case "/api/logs":
			gotLogsQuery = r.URL.Query()
			_ = json.NewEncoder(w).Encode(LogBatch{Events: nil, Next: 99})
		case "/api/logtail":
			gotLogTailQuery = r.URL.Query()
			_ = json.NewEncoder(w).Encode(LogTailBatch{Lines: []string{"a", "b"}, Offset: 10})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	c, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	status, err := c.FetchStatus(ctx)
	if err != nil {
		t.Fatalf("FetchStatus returned error: %v", err)
	}
	if status.PID != 123 || !status.Running {
		t.Fatalf("FetchStatus payload = %#v, want running pid=123", status)
	}

	items, err := c.FetchQueue(ctx)
	if err != nil {
		t.Fatalf("FetchQueue returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != 42 {
		t.Fatalf("FetchQueue items = %#v, want 1 item id=42", items)
	}

	_, err = c.FetchLogs(ctx, LogQuery{
		Since:     7,
		Limit:     13,
		Follow:    true,
		Tail:      true,
		ItemID:    101,
		Level:     "warn",
		Component: "worker",
		Lane:      "fast",
		Request:   "abc",
	})
	if err != nil {
		t.Fatalf("FetchLogs returned error: %v", err)
	}
	if gotLogsQuery.Get("since") != "7" ||
		gotLogsQuery.Get("limit") != "13" ||
		gotLogsQuery.Get("follow") != "1" ||
		gotLogsQuery.Get("tail") != "1" ||
		gotLogsQuery.Get("item") != "101" ||
		gotLogsQuery.Get("level") != "warn" ||
		gotLogsQuery.Get("component") != "worker" ||
		gotLogsQuery.Get("lane") != "fast" ||
		gotLogsQuery.Get("request") != "abc" {
		t.Fatalf("FetchLogs query = %v, want params encoded", gotLogsQuery)
	}

	_, err = c.FetchLogTail(ctx, LogTailQuery{
		ItemID: 101,
		Offset: 5,
		Limit:  50,
		Follow: true,
		WaitMS: 250,
	})
	if err != nil {
		t.Fatalf("FetchLogTail returned error: %v", err)
	}
	if gotLogTailQuery.Get("item") != "101" ||
		gotLogTailQuery.Get("offset") != "5" ||
		gotLogTailQuery.Get("limit") != "50" ||
		gotLogTailQuery.Get("follow") != "1" ||
		gotLogTailQuery.Get("wait_ms") != "250" {
		t.Fatalf("FetchLogTail query = %v, want params encoded", gotLogTailQuery)
	}

	if gotUserAgent == "" || !strings.HasPrefix(gotUserAgent, "flyer/") {
		t.Fatalf("User-Agent = %q, want flyer/*", gotUserAgent)
	}
}

func TestClient_FetchLogTailRequiresItemID(t *testing.T) {
	c, err := NewClient("127.0.0.1:1")
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	_, err = c.FetchLogTail(context.Background(), LogTailQuery{})
	if err == nil {
		t.Fatalf("FetchLogTail returned nil error, want error")
	}
}

func TestClient_HTTPErrorAndDecodeError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{not-json"))
		case "/api/queue":
			http.Error(w, "nope", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	c, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = c.FetchStatus(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("FetchStatus error = %v, want decode response error", err)
	}

	_, err = c.FetchQueue(context.Background())
	if err == nil || !strings.Contains(err.Error(), "returned status 500") {
		t.Fatalf("FetchQueue error = %v, want status 500 error", err)
	}
}
