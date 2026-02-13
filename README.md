# Flyer

A terminal dashboard for monitoring [Spindle](https://github.com/five82/spindle), the disc-ripping daemon. Flyer polls the Spindle API to display queue status, item details, and logs in a single TUI.

Flyer is read-only by design—use the Spindle CLI for retries, clears, or other mutations.

## Features

- **Queue view** — items grouped by status with live counts and filtering
- **Detail view** — metadata, progress, errors, and review flags for the selected item
- **Episode tracker** — per-episode stage summaries for TV box sets
- **Log viewer** — daemon and per-item logs with syntax highlighting
- **Problems view** — aggregated error logs across all items
- **Search** — vim-style `/` search with `n`/`N` navigation and regex support
- **Themes** — Nightfox, Kanagawa, and Slate color schemes

## Installation

```bash
go install github.com/five82/flyer/cmd/flyer@latest
```

Or build from source:

```bash
git clone https://github.com/five82/flyer.git
cd flyer && go build ./cmd/flyer
```

## Requirements

- Go 1.26+
- A running Spindle daemon (default API at `127.0.0.1:7487`)
- For local mode: access to Spindle's config (`~/.config/spindle/config.toml`)
- For remote mode: API endpoint and token (see [Remote Access](#remote-access))

## Usage

```bash
flyer                          # uses default Spindle config
flyer --config /path/to/config.toml  # override config location
flyer --poll 3                 # set refresh interval (default: 2s)
```

Press `h` in the TUI for keyboard shortcuts.

## Remote Access

Flyer can connect to a remote Spindle daemon using CLI flags or environment variables.

**CLI flags:**

```bash
flyer --api http://server:7487 --token mysecrettoken
```

**Environment variables:**

```bash
export FLYER_API_ENDPOINT=http://server:7487
export FLYER_API_TOKEN=mysecrettoken
flyer
```

CLI flags take precedence over environment variables. When neither is set, Flyer auto-discovers the endpoint from the local Spindle config.

**Precedence order:**
1. CLI flags (`--api`, `--token`)
2. Environment variables (`FLYER_API_ENDPOINT`, `FLYER_API_TOKEN`)
3. Local Spindle config (`~/.config/spindle/config.toml`)

See Spindle's [API documentation](https://github.com/five82/spindle/blob/main/docs/api.md) for server-side authentication setup.

## Development

See [AGENTS.md](AGENTS.md) for project structure, development workflow, and contribution guidelines.

Quick commands:

```bash
./check-ci.sh              # run local CI checks
go test ./...              # run tests
go run ./cmd/flyer         # run without installing
```

## License

[GPL-3.0](LICENSE)
