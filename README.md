# flyer

A terminal dashboard for monitoring [Spindle](https://github.com/five82/spindle), the disc-ripping daemon. Flyer polls the Spindle API to display queue status, item details, and logs in a single TUI.

Flyer is read-only by design — use the Spindle CLI for retries, clears, or other mutations.

## Expectations

Flyer is a personal tool built for one workflow, hardware setup, and set of preferences — open source in the spirit of sharing rather than as a general-purpose product. Behavior may change as the workflow evolves, and questions or issues may receive a slow response or none; pull requests are welcome when they fit the project's goals. Expect rough edges.

## Features

- **Dashboard** — queue table with progress and filtering, plus a live NOW band naming the item and task holding each scheduler resource (drive, GPU, encode)
- **Drive availability** — the header always reports the optical drive as AVAILABLE, BUSY, or PAUSED
- **Item inspector** — full-screen drill-in per item: Overview, Episodes (TV box sets), Problems, and Logs tabs
- **Problems triage** — every failed or review item with its lead reason, one keypress from the details
- **Logs** — daemon and per-item logs with highlighting, follow mode, and level/component/lane/request filters
- **Search** — regex log search with `n`/`N`; `/` also filters queue rows by title
- **Themes** — Slate and Nightfox, cycled with `T`

## Install

```bash
go install github.com/five82/flyer/cmd/flyer@latest
```

Requirements:

- Go 1.27.1+
- A running Spindle daemon with `[api].bind` configured
- Local mode: read access to Spindle's config and state directory (`~/.local/state/spindle` by default) for daemon logs
- Remote mode: an API endpoint and bearer token (see [Remote Access](#remote-access))

Flyer runs anywhere the Spindle API is reachable; only the daemon itself is
Linux-bound. To build a source checkout instead:

```bash
git clone https://github.com/five82/flyer.git
cd flyer && go build ./cmd/flyer
```

## Usage

```bash
flyer
```

| Flag | Default | Purpose |
|------|---------|---------|
| `--config` | `$XDG_CONFIG_HOME/spindle/config.toml` | Spindle config to read |
| `--poll` | `2` | Refresh interval, in seconds |
| `--api` | - | Spindle API endpoint (remote mode) |
| `--token` | - | API bearer token (remote mode) |

Press `h` or `?` in the TUI for keyboard shortcuts.

## Remote Access

Flyer reads Spindle's local config by default. Point it at a remote daemon with
flags or environment variables:

| | Flag | Environment variable |
|--|------|----------------------|
| Endpoint | `--api` | `FLYER_API_ENDPOINT` |
| Token | `--token` | `FLYER_API_TOKEN` |

```bash
flyer --api http://server:7487 --token mysecrettoken
```

Resolution order: flags, then environment, then local config. Spindle does not
enable TCP listening by default, so a local setup needs it configured:

```toml
[api]
bind = "127.0.0.1:7487"
token = "choose-a-token"
```

See the [Spindle operator guide](https://github.com/five82/spindle#configure) for
server setup.

## Development

```bash
go run ./cmd/flyer     # run without installing
go test ./...          # run tests
./check-ci.sh          # full local CI: tests, race, lint, govulncheck
./deploy.sh            # build this checkout over the installed binary
```

The deploy script keeps the previous binary beside the installed one and
verifies the installed copy.

See [AGENTS.md](AGENTS.md) for project structure and workflow, and
[docs/design.md](docs/design.md) and [docs/themes.md](docs/themes.md) for the UI
visual language and theme palettes.

## License

[GPL-3.0](LICENSE)
