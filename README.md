<img width="1919" height="905" alt="1" src="https://github.com/user-attachments/assets/a7686995-043c-4bcd-a626-684d07353d14" />
<img width="1917" height="908" alt="2" src="https://github.com/user-attachments/assets/172471fc-4be4-4bda-83b9-34743d1e8b00" />
# gmux

`gmux` is an easy command layer on top of `tmux`.

Goal: keep tmux power, remove tmux complexity.

## What you get

- Simple session model (`1..9`) with quick attach.
- Fast no-prefix keybindings.
- Always-visible status hints.
- Live active-session list in status bar.
- YAML config for keymap/presets.
- One-command cleanup: `kill all`.

## Build

```bash
go build -o gmux ./cmd/gmux
```

## Quick start

```bash
./gmux install
gmux session
```

## Bootstrap (deps + binary)

```bash
./gmux install
```

## CLI

```bash
gmux session         # create next free session (1..9) and attach
gmux 1               # attach session 1
gmux 2               # attach session 2
gmux list            # list managed sessions
gmux kill 1          # kill one managed session
gmux kill all        # kill all managed sessions
gmux rename 2 api    # label session 2 as "api" (shown in status bar)
gmux install         # auto-install tmux + install gmux to /usr/local/bin or /usr/bin
gmux start dev       # attach named session (advanced/optional)
gmux doctor          # print binary + config paths
```

## Key layer (inside tmux, no prefix)

- `Alt+1..9` switch windows
- `Alt+n` new window
- `Alt+h/j/k/l` pane navigation
- `Alt+v` split vertical
- `Alt+s` split horizontal
- `Ctrl+Space` command palette
- `Ctrl-/` quick key help
- `Ctrl+q` or `Ctrl+j` detach client

## Palette commands (`Ctrl+Space`)

- `new`
- `1..9`
- `<text>` rename current window title (example: `api`)
- `splitv`
- `splith`
- `kill`
- `dev`
- `ops`
- `logs`

## Running inside tmux already

If you are already inside a tmux client, `gmux` switches client/session (no nested tmux attach).

## Config

Auto-created:

- `~/.config/gmux/config.yaml`
- `~/.config/gmux/tmux.generated.conf`

Config supports:

- `tmux_bin`
- `default_session`
- `status_template`
- `keymap`
- `presets`

## Status bar

- Active session: black on yellow highlight
- Other sessions: bright white labels
- Wider spacing for readability
- Human labels shown when set (example: `2:api`)
