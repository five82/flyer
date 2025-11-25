package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Config captures the minimal fields Flyer needs from spindle's config.
type Config struct {
	APIBind    string
	LogDir     string
	SocketPath string
}

const (
	defaultConfigPath = "~/.config/spindle/config.toml"
	defaultLogDir     = "~/.local/share/spindle/logs"
	defaultAPIBind    = "127.0.0.1:7487"
)

// Load locates and parses the spindle config, falling back to defaults when missing.
func Load(path string) (Config, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{APIBind: defaultAPIBind, LogDir: defaultLogDir}

	file, err := os.Open(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg.LogDir = mustExpand(defaultLogDir)
			cfg.SocketPath = filepath.Join(cfg.LogDir, "spindle.sock")
			return cfg, nil
		}
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var raw struct {
		APIBind string `toml:"api_bind"`
		LogDir  string `toml:"log_dir"`
	}
	if err := toml.Unmarshal(bytes, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	cfg.APIBind = strings.TrimSpace(raw.APIBind)
	if cfg.APIBind == "" {
		cfg.APIBind = defaultAPIBind
	}

	cfg.LogDir = strings.TrimSpace(raw.LogDir)
	if cfg.LogDir == "" {
		cfg.LogDir = defaultLogDir
	}
	cfg.LogDir = mustExpand(cfg.LogDir)
	cfg.SocketPath = filepath.Join(cfg.LogDir, "spindle.sock")

	return cfg, nil
}

// DaemonLogPath returns the path to the primary spindle log file.
func (c Config) DaemonLogPath() string {
	if strings.TrimSpace(c.LogDir) == "" {
		return mustExpand(defaultLogDir + "/spindle.log")
	}
	return filepath.Join(c.LogDir, "spindle.log")
}

func resolvePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return expandPath(defaultConfigPath)
	}
	return expandPath(path)
}

func mustExpand(path string) string {
	expanded, err := expandPath(path)
	if err != nil {
		return path
	}
	return expanded
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
