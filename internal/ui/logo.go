package ui

import "fmt"

// createLogo returns a compact, single-line wordmark to keep the header short
func createLogo(theme Theme) string {
	return fmt.Sprintf("[%s::b]flyer[-]", theme.Text.Warning)
}
