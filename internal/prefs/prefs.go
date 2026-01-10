// Package prefs handles Flyer user preferences persistence.
// Preferences are stored in ~/.config/flyer/prefs.toml.
package prefs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Prefs holds user preferences for Flyer.
type Prefs struct {
	Theme string `toml:"theme"`
}

const (
	defaultPrefsPath = "~/.config/flyer/prefs.toml"
	defaultTheme     = "Dracula"
)

// DefaultPath returns the default preferences file path.
func DefaultPath() string {
	return defaultPrefsPath
}

// Load reads preferences from the given path, falling back to defaults if missing.
func Load(path string) (Prefs, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return Prefs{Theme: defaultTheme}, nil
	}

	prefs := Prefs{Theme: defaultTheme}

	file, err := os.Open(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return prefs, nil
		}
		return prefs, nil // Graceful degradation
	}
	defer func() { _ = file.Close() }()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return prefs, nil // Graceful degradation
	}

	if err := toml.Unmarshal(bytes, &prefs); err != nil {
		return Prefs{Theme: defaultTheme}, nil // Graceful degradation
	}

	if strings.TrimSpace(prefs.Theme) == "" {
		prefs.Theme = defaultTheme
	}

	return prefs, nil
}

// Save writes preferences to the given path, creating directories as needed.
func Save(path string, p Prefs) error {
	resolved, err := resolvePath(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create prefs dir: %w", err)
	}

	bytes, err := toml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal prefs: %w", err)
	}

	if err := os.WriteFile(resolved, bytes, 0o644); err != nil {
		return fmt.Errorf("write prefs: %w", err)
	}

	return nil
}

func resolvePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return expandPath(defaultPrefsPath)
	}
	return expandPath(path)
}

func expandPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("path is empty")
	}
	if strings.HasPrefix(trimmed, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~"))
	}
	return filepath.Abs(trimmed)
}
