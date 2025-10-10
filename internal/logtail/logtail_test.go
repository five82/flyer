package logtail

import (
	"testing"
)

func TestColorizeLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty line",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "   ",
		},
		{
			name:     "detail line",
			input:    "    - Progress: 50%",
			expected: "    [white:black]Progress: 50%[-:-]",
		},
		{
			name:     "info log with component",
			input:    "2025-10-08 21:01:05 INFO [encoder] – starting encoding",
			expected: "[#808080:black]2025-10-08 21:01:05[-:-] [#5FD75F:black:b]INFO[-:-] [#87AFFF:black][encoder][-:-] [#666666:black]–[-:-] starting encoding",
		},
		{
			name:     "error log with item",
			input:    "2025-10-08 20:05:49 ERROR Item #5 (encoder) – encoding failed",
			expected: "[#808080:black]2025-10-08 20:05:49[-:-] [#FF6B6B:black:b]ERROR[-:-] [#D7AFFF:black]Item #5 (encoder)[-:-] [#666666:black]–[-:-] encoding failed",
		},
		{
			name:     "warn log with component and item",
			input:    "2025-10-08 21:01:05 WARN [encoder] Item #5 (encoder) – slow progress",
			expected: "[#808080:black]2025-10-08 21:01:05[-:-] [#FFD700:black:b]WARN[-:-] [#87AFFF:black][encoder][-:-] [#D7AFFF:black]Item #5 (encoder)[-:-] [#666666:black]–[-:-] slow progress",
		},
		{
			name:     "debug log",
			input:    "2025-10-08 21:01:05 DEBUG – debug message",
			expected: "[#808080:black]2025-10-08 21:01:05[-:-] [#87CEEB:black:b]DEBUG[-:-] [#666666:black]–[-:-] debug message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorizeLine(tt.input)
			if result != tt.expected {
				t.Errorf("ColorizeLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestColorizeLines(t *testing.T) {
	input := []string{
		"2025-10-08 21:01:05 INFO [encoder] – starting encoding",
		"    - Progress: 50%",
		"2025-10-08 21:01:06 ERROR Item #5 (encoder) – encoding failed",
	}

	expected := []string{
		"[#808080:black]2025-10-08 21:01:05[-:-] [#5FD75F:black:b]INFO[-:-] [#87AFFF:black][encoder][-:-] [#666666:black]–[-:-] starting encoding",
		"    [white:black]Progress: 50%[-:-]",
		"[#808080:black]2025-10-08 21:01:06[-:-] [#FF6B6B:black:b]ERROR[-:-] [#D7AFFF:black]Item #5 (encoder)[-:-] [#666666:black]–[-:-] encoding failed",
	}

	result := ColorizeLines(input)

	if len(result) != len(expected) {
		t.Errorf("ColorizeLines() returned %d lines, want %d", len(result), len(expected))
	}

	for i, line := range result {
		if line != expected[i] {
			t.Errorf("ColorizeLines()[%d] = %q, want %q", i, line, expected[i])
		}
	}
}
