# Flyer

A terminal dashboard for monitoring [Spindle](https://github.com/five82/spindle), the disc-ripping daemon. Flyer polls the Spindle API and tails log files to display queue status, item details, and logs in a single TUI.

Flyer is read-only by design—use the Spindle CLI for retries, clears, or other mutations.

## Features

- **Queue view** — items grouped by status with live counts
- **Detail view** — metadata, progress, errors, and review flags for the selected item
- **Episode tracker** — per-episode stage summaries for TV box sets
- **Log viewer** — daemon and per-item logs with syntax highlighting
- **Search** — vim-style `/` search with `n`/`N` navigation and regex support

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

- Go 1.25+
- A running Spindle daemon (default API at `127.0.0.1:7487`)
- Access to Spindle's config (`~/.config/spindle/config.toml`) and log directories

## Usage

```bash
flyer                          # uses default Spindle config
flyer --config /path/to/config.toml  # override config location
flyer --poll 3                 # set refresh interval (default: 2s)
```

Press `h` in the TUI for the help overlay, or see [docs/keybindings.md](docs/keybindings.md) for the full reference.

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
