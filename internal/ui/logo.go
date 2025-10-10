package ui

import (
	"os/exec"
	"strings"
)

// createLogo generates the flyer logo using figlet or fallback
func createLogo() string {
	// Try to use figlet for ASCII art
	cmd := exec.Command("figlet", "-f", "slant", "flyer")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		// Apply orange color to figlet output (k9s-style logo color)
		return applyOrangeColor(string(output))
	}

	// Fallback: simple orange FLYER text (k9s-style)
	return "[orange]FLYER[-]"
}

// applyOrangeColor applies orange color to text using tview color tags (k9s-style)
func applyOrangeColor(text string) string {
	color := "[orange]"
	var result strings.Builder
	lines := strings.Split(text, "\n")

	for lineIdx, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		result.WriteString(color) // Start orange color for this line
		for _, r := range line {
			result.WriteRune(r)
		}
		result.WriteString("[-]") // End orange color for this line

		if lineIdx < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}
