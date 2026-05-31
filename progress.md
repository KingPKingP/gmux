# Progress

## Status

Stable baseline locked (v1-local). Core easy-flow CLI implemented and working.

## Stable baseline (current)

- Build:
  - `go build -o gmux ./cmd/gmux`
- Bootstrap:
  - `./gmux install`
- Session flow:
  - `./gmux session`
  - `./gmux <1..9>`
  - `./gmux list`
  - `./gmux kill <1..9|all>`
  - `./gmux rename <n> <label>`
- Status behavior:
  - Active session highlighted.
  - Labels shown as `n:label` when set.
  - `gmux list` shows `session: n (label)` when label exists.
- Detach:
  - `Ctrl+q` or `Ctrl+j`
- Fast keys:
  - `Alt+1..9`, `Alt+n`, `Alt+h/j/k/l`, `Alt+v`, `Alt+s`
  - `Ctrl+Space` palette
  - `Ctrl-/` quick help

## Done

- Switched architecture to tmux-backed wrapper.
- Added numeric session UX:
  - `gmux session` allocates next free `1..9`.
  - `gmux <n>` attaches numeric session.
  - `gmux kill <n>`.
  - `gmux kill all`.
- Added no-prefix key layer in generated tmux config:
  - `Alt+1..9`, `Alt+n`, `Alt+h/j/k/l`, `Alt+v`, `Alt+s`.
- Added quick detach:
  - `Ctrl+q`, `Ctrl+j` (detach client only).
- Added status line template and YAML config loader.
- Added presets mapping (`dev`, `ops`, `logs`).
- Added palette parser command (`internal/tmux/palette.go`) and tmux binding hook.
- Added `kill all` command.
- Added session labels: `gmux rename <n> <name>` -> shown in status bar.
- Added dynamic status-right session list with active highlight.
- Added quick help overlay: `Ctrl-/` (and `Ctrl-_` fallback).
- Added `gmux install`:
  - installs tmux with detected package manager,
  - installs gmux binary to `/usr/local/bin` or `/usr/bin` (sudo fallback).
- Verified Go build:
  - `go build ./...`
  - `go build -o gmux ./cmd/gmux`

## In Progress / Needs real-machine validation

- End-to-end key behavior in each terminal app (Alt/Ctrl mapping can differ).
- `Ctrl+Space` binding reliability across terminal emulators.
- `gmux install` package-manager coverage for uncommon distros.

## Known constraints

- Requires `tmux` installed locally.
- `gmux install` may require `sudo` for system path copy.
- This sandbox could not run full live tmux attach tests (`tmux` binary missing).
- Very long session+label lists can still be limited by terminal width.

## Next recommended steps

1. Run full manual test on your machine:
   - `./gmux session`
   - `Alt+n`, `Alt+1..9`, pane keys
   - `Ctrl+Space` palette commands
   - `Ctrl+q` detach
2. Run `./gmux install` on a clean machine and validate full bootstrap.
3. If any key does not trigger, capture terminal app name and exact key behavior and add terminal-specific fallback bindings.
