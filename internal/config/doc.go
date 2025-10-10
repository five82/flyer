// Package config handles loading and parsing Spindle daemon configuration files.
//
// # Overview
//
// This package reads Spindle's TOML configuration to discover the daemon's API
// endpoint and log file locations. Flyer uses a minimal subset of the full
// Spindle configuration, only extracting fields needed for monitoring.
//
// # Configuration Discovery
//
// The Load function follows this resolution order:
//
//  1. If a path is explicitly provided, use it
//  2. Otherwise, use ~/.config/spindle/config.toml (default)
//  3. If the config file doesn't exist, fall back to hardcoded defaults
//  4. If the file exists but fields are missing/empty, use defaults
//
// # Default Values
//
//   - Config file: ~/.config/spindle/config.toml
//   - API endpoint: 127.0.0.1:7487
//   - Log directory: ~/.local/share/spindle/logs
//   - Daemon log: <log_dir>/spindle.log
//   - Socket path: <log_dir>/spindle.sock
//
// # Configuration Fields
//
// The Config struct contains only the fields Flyer needs:
//
//   - APIBind: HTTP API endpoint (host:port) for Spindle daemon
//   - LogDir: Directory containing daemon and item-specific logs
//   - SocketPath: Unix socket path (derived from LogDir)
//
// # TOML Format
//
// Example spindle config.toml:
//
//	api_bind = "127.0.0.1:7487"
//	log_dir = "~/.local/share/spindle/logs"
//
// Both fields are optional. Tilde expansion is performed automatically.
//
// # Path Expansion
//
// The package handles several path formats:
//
//   - Absolute paths: Used as-is ("/var/log/spindle")
//   - Tilde paths: Expanded to home directory ("~/.config/spindle")
//   - Relative paths: Converted to absolute based on current directory
//
// Path expansion is performed for:
//   - Config file location
//   - log_dir field
//   - Derived paths (daemon log, socket)
//
// # Error Handling
//
// Load returns errors for:
//   - Path expansion failures (e.g., cannot determine home directory)
//   - File read errors (except os.ErrNotExist, which triggers defaults)
//   - TOML parsing errors
//
// Missing config files are NOT an error - defaults are used instead.
// This allows Flyer to work out-of-the-box without configuration.
//
// # Usage Example
//
//	// Use default config path
//	cfg, err := config.Load("")
//	if err != nil {
//		log.Fatalf("failed to load config: %v", err)
//	}
//
//	// Use explicit config path
//	cfg, err := config.Load("/etc/spindle/config.toml")
//	if err != nil {
//		log.Fatalf("failed to load config: %v", err)
//	}
//
//	// Access configuration
//	client, err := spindle.NewClient(cfg.APIBind)
//	logPath := cfg.DaemonLogPath()
//
// # Design Philosophy
//
// This package follows the principle of sensible defaults. Flyer should work
// immediately on a system with Spindle installed in the default location,
// without requiring any configuration file to exist.
//
// The config package is read-only and stateless - it loads configuration
// once at startup and returns an immutable Config struct. No global state
// or singleton patterns are used.
//
// # Testing Considerations
//
// When testing code that uses this package:
//   - Provide explicit config paths to avoid dependency on user's home directory
//   - Use Config struct directly rather than Load() for unit tests
//   - Test path expansion edge cases (missing home dir, symlinks, etc.)
package config
