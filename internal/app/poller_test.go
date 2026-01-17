package app

import (
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	baseInterval := 2 * time.Second

	tests := []struct {
		name     string
		failures int
		want     time.Duration
	}{
		{"zero failures", 0, 2 * time.Second},
		{"negative failures", -1, 2 * time.Second},
		{"one failure", 1, 4 * time.Second},
		{"two failures", 2, 8 * time.Second},
		{"three failures", 3, 16 * time.Second},
		{"four failures capped", 4, 30 * time.Second}, // Would be 32s, capped to 30s
		{"many failures capped", 10, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateBackoff(tt.failures, baseInterval)
			if got != tt.want {
				t.Errorf("calculateBackoff(%d, %v) = %v, want %v", tt.failures, baseInterval, got, tt.want)
			}
		})
	}
}

func TestCalculateBackoff_MaxCap(t *testing.T) {
	// Verify that backoff never exceeds maxBackoff regardless of input
	baseInterval := 2 * time.Second
	for failures := 0; failures <= 20; failures++ {
		got := calculateBackoff(failures, baseInterval)
		if got > maxBackoff {
			t.Errorf("calculateBackoff(%d, %v) = %v, exceeds maxBackoff %v", failures, baseInterval, got, maxBackoff)
		}
	}
}
