package ui

import (
	"testing"
	"time"
)

func TestHumanizeDuration(t *testing.T) {
	cases := []struct {
		name string
		in   int64 // seconds
		want string
	}{
		{"negative", -5, "now"},
		{"subsecond", 0, "now"},
		{"seconds", 12, "12s"},
		{"minutes", 61, "1m"},
		{"hours_only", 2*60*60 + 10, "2h"},
		{"hours_minutes", 2*60*60 + 3*60, "2h 3m"},
		{"days", 24 * 60 * 60, "1d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := humanizeDuration(timeSeconds(tc.in))
			if got != tc.want {
				t.Fatalf("humanizeDuration(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTruncateMiddle(t *testing.T) {
	if got := truncateMiddle("  ", 10); got != "" {
		t.Fatalf("truncateMiddle blank = %q, want empty", got)
	}
	if got := truncateMiddle("abcd", 2); got != "ab" {
		t.Fatalf("truncateMiddle limit<=3 = %q, want ab", got)
	}
	got := truncateMiddle("a/b/c/d/e", 7)
	if got == "a/b/c/d/e" {
		t.Fatalf("expected truncation")
	}
	if len([]rune(got)) > 7 {
		t.Fatalf("got %q (%d runes), want <=7", got, len([]rune(got)))
	}
}

func TestFormatBytes(t *testing.T) {
	if got := formatBytes(999); got != "999 B" {
		t.Fatalf("formatBytes = %q, want 999 B", got)
	}
	if got := formatBytes(1024); got != "1.00 KiB" {
		t.Fatalf("formatBytes = %q, want 1.00 KiB", got)
	}
	if got := formatBytes(1024 * 1024); got != "1.00 MiB" {
		t.Fatalf("formatBytes = %q, want 1.00 MiB", got)
	}
}

func TestIsRipCacheHitMessage(t *testing.T) {
	if !isRipCacheHitMessage("Rip cache hit; skipping MakeMKV rip") {
		t.Fatalf("expected message to match rip cache hit")
	}
	if isRipCacheHitMessage("whisperx transcript cache hit") {
		t.Fatalf("did not expect whisperx cache hit to match rip cache hit")
	}
}

func timeSeconds(sec int64) time.Duration {
	return time.Duration(sec) * time.Second
}
