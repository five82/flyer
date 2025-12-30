package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

func TestComposeSubject(t *testing.T) {
	if got := composeSubject(0, "stage"); got != "stage" {
		t.Fatalf("composeSubject = %q, want stage", got)
	}
	if got := composeSubject(10, ""); got != "Item #10" {
		t.Fatalf("composeSubject = %q, want Item #10", got)
	}
	if got := composeSubject(10, "rip"); got != "Item #10 (rip)" {
		t.Fatalf("composeSubject = %q, want Item #10 (rip)", got)
	}
}

func TestFormatLogEvent(t *testing.T) {
	oldLocal := time.Local
	time.Local = time.FixedZone("TestLocal", -5*60*60)
	defer func() {
		time.Local = oldLocal
	}()

	evt := spindle.LogEvent{
		Timestamp: "2025-12-13T10:11:12Z",
		Level:     "warn",
		Message:   " hello ",
		Component: "worker",
		Stage:     "rip",
		ItemID:    42,
		Details: []spindle.DetailField{
			{Label: "path", Value: "/tmp/a"},
		},
	}
	got := formatLogEvent(evt)
	if wantSub := "2025-12-13 05:11:12 WARN [worker] Item #42 (rip) – hello"; !strings.Contains(got, wantSub) {
		t.Fatalf("formatLogEvent = %q, want it to contain %q", got, wantSub)
	}
	if !strings.Contains(got, "\n    - path: /tmp/a") {
		t.Fatalf("formatLogEvent missing details: %q", got)
	}
}
