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
		// Apply yellow color to figlet output
		return applyYellowColor(string(output))
	}

	// Fallback: simple yellow FLYER text
	return "[yellow]FLYER[-]"
}

// applyYellowColor applies yellow color to text using tview color tags
func applyYellowColor(text string) string {
	color := "[yellow]"
	var result strings.Builder
	lines := strings.Split(text, "\n")

	for lineIdx, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		result.WriteString(color) // Start yellow color for this line
		for _, r := range line {
			result.WriteRune(r)
		}
		result.WriteString("[-]") // End yellow color for this line

		if lineIdx < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}
