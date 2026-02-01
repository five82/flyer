# AGENTS.md

This file provides guidance when working with code in this repository.

CLAUDE.md and GEMINI.md are symlinks to this file so all agent guidance stays in one place.
Do not modify this header.

## TL;DR

- Do not run `git commit` or `git push` unless explicitly instructed.
- Run `./check-ci.sh` before handing work back.
- Tests must not read real home directory or Spindle config - use `t.TempDir()` + `t.Setenv("HOME", ...)`.

## Project

Flyer is a **read-only TUI** for monitoring Spindle. Single-developer hobby project - avoid over-engineering.

## Related Repos

| Repo | Path | Role |
|------|------|------|
| flyer | `~/projects/flyer/` | Read-only TUI for Spindle (this repo) |
| spindle | `~/projects/spindle/` | Daemon + CLI; Flyer polls its `api_bind` endpoint |
| drapto | `~/projects/drapto/` | Encoder invoked by Spindle; Flyer does not call directly |

GitHub: [flyer](https://github.com/five82/flyer) | [spindle](https://github.com/five82/spindle) | [drapto](https://github.com/five82/drapto)

## Build, Test, Lint

```bash
go run ./cmd/flyer     # Run TUI
go test ./...          # Test
golangci-lint run      # Lint
./check-ci.sh          # Full CI (recommended before handoff)
```

## Scope Constraints

Flyer is intentionally limited:
- **Read-only**: No queue mutations, retries, or clears
- **Single operator**: No auth, no multi-profile

When considering features, ask: "Does this solve a real problem for daily use?" If not, skip it.
