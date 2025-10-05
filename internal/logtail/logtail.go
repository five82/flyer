package logtail

import (
	"bufio"
	"errors"
	"fmt"
	"os"
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
