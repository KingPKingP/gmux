# HOWTO

## 1. Install requirements

`gmux` needs `tmux` installed and available in `PATH`.

Check:

```bash
tmux -V
```

Or let gmux install dependencies + binary:

```bash
./gmux install
```

## 2. Build gmux

```bash
go build -o gmux ./cmd/gmux
```

## 3. First run

Create and attach first numeric session:

```bash
./gmux session
```

Detach from session:

- `Ctrl+q` or `Ctrl+j`

Re-attach later:

```bash
./gmux 1
```

## 4. Work with multiple sessions

```bash
./gmux session   # creates 1
./gmux session   # creates 2
./gmux session   # creates 3
./gmux list
./gmux 2
```

Kill one:

```bash
./gmux kill 2
```

Rename session label (shown in status bar):

```bash
./gmux rename 1 devapi
```

Kill all:

```bash
./gmux kill all
```

## 5. Use fast keys inside session

- `Alt+1..9` switch windows
- `Alt+n` new window
- `Alt+h/j/k/l` move pane focus
- `Alt+v` vertical split
- `Alt+s` horizontal split
- `Ctrl+Space` palette
- `Ctrl-/` quick help
- `Ctrl+q` or `Ctrl+j` detach without killing session

Palette examples:

- `new`
- `2`
- `api` (renames current window title to `api`)
- `splitv`
- `splith`
- `kill`
- `dev`
- `ops`
- `logs`

## 6.1 If already inside tmux

`gmux` switches to the target session/client instead of nesting tmux.

## 6. Customize config

Default path:

`~/.config/gmux/config.yaml`

Useful keys:

- `default_session`
- `status_template`
- `keymap.new_window`
- `keymap.command_palette`
- `presets`

## 7. Status bar behavior

- Shows all active sessions with spacing.
- Active session is highlighted (black/yellow).
- Other sessions are bright white.
- If label set, shows `n:label` (example `2:api`).

## 8. Troubleshooting

### tmux not found

Error like:

`tmux binary not found`

Fix:

1. Install tmux.
2. Or set `tmux_bin` in config to full path.

### Alt keys not detected by terminal

Some terminals send `Esc+key` differently.

Try:

1. Use another terminal profile.
2. Rebind keys in config.
3. Keep `Ctrl+Space` palette as fallback control path.

### Session stuck or bad layout

Reset quickly:

```bash
./gmux kill all
```
