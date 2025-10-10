package logtail

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Read returns at most maxLines from the end of the file at path.
func Read(path string, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer file.Close()

	ring := make([]string, maxLines)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	count := 0
	idx := 0
	for scanner.Scan() {
		ring[idx] = scanner.Text()
		idx = (idx + 1) % maxLines
		if count < maxLines {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}

	lines := make([]string, count)
	if count == maxLines {
		for i := 0; i < count; i++ {
			lines[i] = ring[(idx+i)%maxLines]
		}
	} else {
		copy(lines, ring[:count])
	}
	return lines, nil
}

// ColorizeLine applies syntax highlighting to a log line using tview color codes
func ColorizeLine(line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}

	// Detail lines (indented with spaces and starting with -)
	if strings.HasPrefix(line, "    -") {
		return fmt.Sprintf("    [white]%s[-]", line[6:])
	}

	// Parse log line components more carefully
	// Pattern: timestamp LEVEL [component] Item #X (name) – message
	timestampRegex := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)
	levelRegex := regexp.MustCompile(`\b(INFO|WARN|ERROR|DEBUG)\b`)
	componentRegex := regexp.MustCompile(`\[([^\]]+)\]`)
	itemRegex := regexp.MustCompile(`Item #\d+ \([^)]+\)`)
	separatorRegex := regexp.MustCompile(`\s*–\s*`)

	var result strings.Builder
	remaining := line

	// Extract and colorize timestamp (dim gray)
	if matches := timestampRegex.FindStringSubmatchIndex(remaining); len(matches) > 0 {
		start, end := matches[2], matches[3]
		result.WriteString("[#666666]")
		result.WriteString(remaining[start:end])
		result.WriteString("[-]")
		remaining = remaining[end:]
	}

	// Extract and colorize log level
	if matches := levelRegex.FindStringSubmatchIndex(remaining); len(matches) > 0 {
		start, end := matches[2], matches[3]
		level := remaining[start:end]
		var color string
		switch level {
		case "INFO":
			color = "green" // Standard for info
		case "WARN":
			color = "yellow" // Standard for warnings
		case "ERROR":
			color = "red" // Standard for errors
		case "DEBUG":
			color = "cyan" // Standard for debug
		default:
			color = "white"
		}
		result.WriteString(" [")
		result.WriteString(color)
		result.WriteString(":b]") // Bold for emphasis
		result.WriteString(level)
		result.WriteString("[-]")
		remaining = remaining[end:]
	}

	// Extract and colorize component (subtle blue)
	if matches := componentRegex.FindStringSubmatchIndex(remaining); len(matches) > 0 {
		start, end := matches[0], matches[1]
		component := remaining[start:end]
		result.WriteString(" [#6495ED]") // Cornflower blue
		result.WriteString(component)
		result.WriteString("[-]")
		remaining = remaining[end:]
	}

	// Extract and colorize item info (subtle magenta)
	if matches := itemRegex.FindStringSubmatchIndex(remaining); len(matches) > 0 {
		start, end := matches[0], matches[1]
		item := remaining[start:end]
		result.WriteString(" [#DA70D6]") // Orchid
		result.WriteString(item)
		result.WriteString("[-]")
		remaining = remaining[end:]
	}

	// Handle separator and message
	if separatorRegex.MatchString(remaining) {
		parts := separatorRegex.Split(remaining, 2)
		if len(parts) == 2 {
			result.WriteString(" [#AAAAAA]–[-] ") // Light gray separator
			result.WriteString(parts[1])
		} else {
			result.WriteString(remaining)
		}
	} else {
		result.WriteString(remaining)
	}

	return result.String()
}

// ColorizeLines applies syntax highlighting to multiple log lines
func ColorizeLines(lines []string) []string {
	colorized := make([]string, len(lines))
	for i, line := range lines {
		colorized[i] = ColorizeLine(line)
	}
	return colorized
}
