package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_MissingConfigFallsBackToDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := Load(filepath.Join(home, "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIBind != defaultAPIBind {
		t.Fatalf("APIBind = %q, want %q", cfg.APIBind, defaultAPIBind)
	}

	wantLogDir, err := expandPath(defaultLogDir)
	if err != nil {
		t.Fatalf("expandPath(defaultLogDir) returned error: %v", err)
	}
	if cfg.LogDir != wantLogDir {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, wantLogDir)
	}
	if cfg.SocketPath != filepath.Join(wantLogDir, "spindle.sock") {
		t.Fatalf("SocketPath = %q, want %q", cfg.SocketPath, filepath.Join(wantLogDir, "spindle.sock"))
	}
}

func TestLoad_ParsesAndTrimsConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
api_bind = "  10.0.0.5:9999  "
log_dir = "  ~/.spindle/logs  "
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIBind != "10.0.0.5:9999" {
		t.Fatalf("APIBind = %q, want %q", cfg.APIBind, "10.0.0.5:9999")
	}
	if !strings.HasPrefix(cfg.LogDir, home) {
		t.Fatalf("LogDir = %q, want it under HOME %q", cfg.LogDir, home)
	}
	if cfg.SocketPath != filepath.Join(cfg.LogDir, "spindle.sock") {
		t.Fatalf("SocketPath = %q, want %q", cfg.SocketPath, filepath.Join(cfg.LogDir, "spindle.sock"))
	}
}

func TestLoad_EmptyValuesUseDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
api_bind = "   "
log_dir = ""
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIBind != defaultAPIBind {
		t.Fatalf("APIBind = %q, want %q", cfg.APIBind, defaultAPIBind)
	}
	wantLogDir, err := expandPath(defaultLogDir)
	if err != nil {
		t.Fatalf("expandPath(defaultLogDir) returned error: %v", err)
	}
	if cfg.LogDir != wantLogDir {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, wantLogDir)
	}
}

func TestLoad_InvalidTOMLFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`api_bind = [`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatalf("Load returned nil error, want parse error")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("Load error = %q, want it to mention parse config", err.Error())
	}
}

func TestExpandPath_ExpandsTildeAndReturnsAbs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := expandPath("~/a/b")
	if err != nil {
		t.Fatalf("expandPath returned error: %v", err)
	}
	want := filepath.Join(home, "a/b")
	if got != want {
		t.Fatalf("expandPath = %q, want %q", got, want)
	}
}

func TestExpandPath_EmptyErrors(t *testing.T) {
	if _, err := expandPath("   "); err == nil {
		t.Fatalf("expandPath returned nil error, want error")
	}
}

func TestDaemonLogPath_DefaultsWhenLogDirEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var cfg Config
	got := cfg.DaemonLogPath()
	if !strings.HasPrefix(got, home) {
		t.Fatalf("DaemonLogPath = %q, want it under HOME %q", got, home)
	}
	if !strings.HasSuffix(got, filepath.FromSlash("/spindle.log")) {
		t.Fatalf("DaemonLogPath = %q, want it to end with /spindle.log", got)
	}
}
