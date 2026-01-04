package ui

import "strings"

// truncate shortens a string to the given limit, adding ellipsis if needed.
func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

// truncateMiddle shortens a string by removing characters from the middle,
// preserving both the beginning and end. For paths, it preserves file extensions.
func truncateMiddle(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || value == "" {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}

	ellipsis := []rune("…/")
	if limit <= len(ellipsis) {
		return string(runes[:limit])
	}

	// Smart path truncation: preserve file extension if it looks like a path
	isPath := strings.Contains(value, "/") || strings.Contains(value, "\\")
	if isPath {
		// Find the extension
		lastDot := strings.LastIndex(value, ".")
		lastSlash := maxInt(strings.LastIndex(value, "/"), strings.LastIndex(value, "\\"))

		// Only preserve extension if the dot comes after the last slash
		if lastDot > lastSlash && lastDot > 0 {
			ext := value[lastDot:]
			extRunes := []rune(ext)

			// Only preserve if extension is reasonable length (< 10 chars)
			if len(extRunes) < 10 && len(extRunes) < limit/2 {
				baseName := value[:lastDot]
				baseRunes := []rune(baseName)

				// Calculate space for base (accounting for ellipsis and extension)
				baseLimit := limit - len(extRunes) - len(ellipsis)
				if baseLimit > 0 && len(baseRunes) > baseLimit {
					// Truncate base from middle, preserving extension
					prefix := baseLimit / 2
					suffix := baseLimit - prefix
					return string(baseRunes[:prefix]) + string(ellipsis) + string(baseRunes[len(baseRunes)-suffix:]) + ext
				}
			}
		}
	}

	// Default middle truncation
	keep := limit - len(ellipsis)
	prefix := keep / 2
	suffix := keep - prefix
	return string(runes[:prefix]) + string(ellipsis) + string(runes[len(runes)-suffix:])
}

// titleCase converts an underscore-separated string to title case.
func titleCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}

// padRight pads a string with spaces to the given width.
func padRight(s string, width int) string {
	if width <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(r))
}

// maxInt returns the larger of two integers.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ternary returns a if cond is true, otherwise b.
func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
