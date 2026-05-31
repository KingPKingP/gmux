package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gmux/internal/config"
)

type labelCacheEntry struct {
	value string
	at    time.Time
}

var (
	labelCacheMu sync.Mutex
	labelCache   = map[string]labelCacheEntry{}
	labelTTL     = 2 * time.Second
)

func Start(cfg config.Config, target string) error {
	if err := ensureTmux(cfg.TmuxBin); err != nil {
		return err
	}
	session := target
	if session == "" {
		session = cfg.DefaultSession
	}
	confPath, err := generateConfig(cfg)
	if err != nil {
		return err
	}
	if err := ensureSession(cfg, session); err != nil {
		return err
	}
	if err := sourceConfig(cfg, confPath); err != nil {
		return err
	}
	if err := applyPresetIfAny(cfg, session, target); err != nil {
		return err
	}
	return attach(cfg, session, confPath)
}

func ParseNumericSession(arg string) (string, bool) {
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > 9 {
		return "", false
	}
	return strconv.Itoa(n), true
}

func NextNumericSession(cfg config.Config, max int) (string, error) {
	existing, err := listManagedSessions(cfg)
	if err != nil {
		return "", err
	}
	used := map[string]bool{}
	for _, s := range existing {
		used[s] = true
	}
	for i := 1; i <= max; i++ {
		k := strconv.Itoa(i)
		if !used[k] {
			return k, nil
		}
	}
	return "", fmt.Errorf("no free numeric session slots (1..%d)", max)
}

func List(cfg config.Config) error {
	lines, err := listManagedSessions(cfg)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		fmt.Println("no sessions")
		return nil
	}
	sort.Strings(lines)
	for _, s := range lines {
		fmt.Println(s)
	}
	return nil
}

func listManagedSessions(cfg config.Config) ([]string, error) {
	if err := ensureTmux(cfg.TmuxBin); err != nil {
		return nil, err
	}
	cmd := exec.Command(cfg.TmuxBin, "list-sessions", "-F", "#{session_name}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.ToLower(string(out))
		if strings.Contains(msg, "no server running") || strings.Contains(msg, "error connecting to") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %s", strings.TrimSpace(string(out)))
	}
	lines := strings.Fields(string(out))
	filtered := make([]string, 0, len(lines))
	for _, s := range lines {
		if strings.HasPrefix(s, "gmux-") {
			filtered = append(filtered, strings.TrimPrefix(s, "gmux-"))
		}
	}
	return filtered, nil
}

func Kill(cfg config.Config, session string) error {
	if session == "all" {
		return KillAll(cfg)
	}
	resolved, err := resolveSessionRef(cfg, session)
	if err != nil {
		return err
	}
	s := managedSessionName(resolved)
	cmd := exec.Command(cfg.TmuxBin, "kill-session", "-t", s)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux kill-session: %s", strings.TrimSpace(string(out)))
	}
	fmt.Printf("killed session: %s\n", resolved)
	return nil
}

func KillAll(cfg config.Config) error {
	sessions, err := listManagedSessions(cfg)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Println("no sessions")
		return nil
	}
	for _, s := range sessions {
		full := managedSessionName(s)
		cmd := exec.Command(cfg.TmuxBin, "kill-session", "-t", full)
		out, kerr := cmd.CombinedOutput()
		if kerr != nil {
			return fmt.Errorf("tmux kill-session %s: %s", s, strings.TrimSpace(string(out)))
		}
		fmt.Printf("killed session: %s\n", s)
	}
	return nil
}

func Doctor(cfg config.Config) error {
	p, err := exec.LookPath(cfg.TmuxBin)
	if err != nil {
		return err
	}
	conf, err := gmuxTmuxConfPath()
	if err != nil {
		return err
	}
	fmt.Printf("tmux bin: %s\n", p)
	fmt.Printf("gmux config: %s\n", config.Path())
	fmt.Printf("generated tmux conf: %s\n", conf)
	return nil
}

func StatusLine(cfg config.Config, current string) error {
	sessions, err := listManagedSessions(cfg)
	if err != nil {
		// Keep status bar resilient even if tmux is restarting.
		fmt.Print("sessions: -")
		return nil
	}
	if len(sessions) == 0 {
		fmt.Print("sessions: -")
		return nil
	}
	sort.Strings(sessions)
	active := strings.TrimPrefix(current, "gmux-")
	parts := make([]string, 0, len(sessions))
	for _, s := range sessions {
		label := sessionLabel(cfg, s)
		display := s
		if label != "" && label != s {
			display = s + ":" + label
		}
		if s == active {
			parts = append(parts, "#[fg=black,bg=yellow,bold]  "+display+"  #[default]")
		} else {
			parts = append(parts, "#[fg=white,bold] "+display+" #[default]")
		}
	}
	fmt.Printf("sessions:  %s", strings.Join(parts, "   "))
	return nil
}

func Rename(cfg config.Config, session string, nameParts []string) error {
	name := strings.TrimSpace(strings.Join(nameParts, " "))
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	s := managedSessionName(session)
	cmd := exec.Command(cfg.TmuxBin, "set-option", "-t", s, "@gmux_label", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux set-option @gmux_label: %s", strings.TrimSpace(string(out)))
	}
	labelCacheMu.Lock()
	labelCache[session] = labelCacheEntry{value: name, at: time.Now()}
	labelCacheMu.Unlock()
	fmt.Printf("labeled session %s -> %s\n", session, name)
	return nil
}

func sessionLabel(cfg config.Config, session string) string {
	now := time.Now()
	labelCacheMu.Lock()
	if e, ok := labelCache[session]; ok && now.Sub(e.at) < labelTTL {
		labelCacheMu.Unlock()
		return e.value
	}
	labelCacheMu.Unlock()

	s := managedSessionName(session)
	cmd := exec.Command(cfg.TmuxBin, "show-option", "-qv", "-t", s, "@gmux_label")
	out, err := cmd.CombinedOutput()
	if err != nil {
		labelCacheMu.Lock()
		labelCache[session] = labelCacheEntry{value: "", at: now}
		labelCacheMu.Unlock()
		return ""
	}
	v := strings.TrimSpace(string(out))
	labelCacheMu.Lock()
	labelCache[session] = labelCacheEntry{value: v, at: now}
	labelCacheMu.Unlock()
	return v
}

func resolveSessionRef(cfg config.Config, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("missing session reference")
	}
	sessions, err := listManagedSessions(cfg)
	if err != nil {
		return "", err
	}
	for _, s := range sessions {
		if s == ref {
			return s, nil
		}
	}
	for _, s := range sessions {
		if sessionLabel(cfg, s) == ref {
			return s, nil
		}
	}
	return "", fmt.Errorf("session not found: %s", ref)
}

func ensureTmux(tmuxBin string) error {
	_, err := exec.LookPath(tmuxBin)
	if err != nil {
		return fmt.Errorf("tmux binary not found: %q (install tmux or set tmux_bin in config)", tmuxBin)
	}
	return nil
}

func ensureSession(cfg config.Config, session string) error {
	s := managedSessionName(session)
	check := exec.Command(cfg.TmuxBin, "has-session", "-t", s)
	if err := check.Run(); err == nil {
		return nil
	}
	cmd := exec.Command(cfg.TmuxBin, "new-session", "-d", "-s", s)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func applyPresetIfAny(cfg config.Config, session string, target string) error {
	s := managedSessionName(session)
	presetCmd, ok := cfg.Presets[target]
	if !ok || strings.TrimSpace(presetCmd) == "" {
		return nil
	}
	cmd := exec.Command(cfg.TmuxBin, "send-keys", "-t", s+":1", presetCmd, "C-m")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux send-keys preset: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func attach(cfg config.Config, session, confPath string) error {
	s := managedSessionName(session)
	var cmd *exec.Cmd
	if os.Getenv("TMUX") != "" {
		// Already inside tmux: switch current client instead of nesting.
		cmd = exec.Command(cfg.TmuxBin, "switch-client", "-t", s)
	} else {
		cmd = exec.Command(cfg.TmuxBin, "-f", confPath, "attach-session", "-t", s)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func sourceConfig(cfg config.Config, confPath string) error {
	cmd := exec.Command(cfg.TmuxBin, "source-file", confPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.ToLower(string(out))
		if strings.Contains(msg, "no server running") || strings.Contains(msg, "error connecting to") {
			return nil
		}
		return fmt.Errorf("tmux source-file: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func managedSessionName(session string) string {
	if strings.HasPrefix(session, "gmux-") {
		return session
	}
	return "gmux-" + session
}

func gmuxTmuxConfPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(home, ".config", "gmux", "tmux.generated.conf")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	return p, nil
}

func generateConfig(cfg config.Config) (string, error) {
	p, err := gmuxTmuxConfPath()
	if err != nil {
		return "", err
	}
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		exe = "gmux"
	}
	exe = strings.ReplaceAll(exe, "\\", "\\\\")
	exe = strings.ReplaceAll(exe, "\"", "\\\"")

	var b strings.Builder
	b.WriteString("set -g prefix C-b\n")
	b.WriteString("set -g status on\n")
	b.WriteString("set -g status-interval 1\n")
	b.WriteString("set -g status-justify left\n")
	b.WriteString(fmt.Sprintf("set -g status-left '%s'\n", escapeSingle(cfg.StatusTemplate)))
	b.WriteString(fmt.Sprintf("set -g status-right '#(\"%s\" statusline #{session_name})'\n", exe))
	b.WriteString("set -g mouse on\n")
	b.WriteString("set -g history-limit 50000\n")
	b.WriteString("unbind C-b\n")
	b.WriteString("set -g prefix None\n")
	for i := 1; i <= 9; i++ {
		b.WriteString(fmt.Sprintf("bind-key -n M-%d select-window -t %d\n", i, i))
	}
	b.WriteString(fmt.Sprintf("bind-key -n %s new-window\n", cfg.Keymap.NewWindow))
	b.WriteString(fmt.Sprintf("bind-key -n %s select-pane -L\n", cfg.Keymap.PaneLeft))
	b.WriteString(fmt.Sprintf("bind-key -n %s select-pane -D\n", cfg.Keymap.PaneDown))
	b.WriteString(fmt.Sprintf("bind-key -n %s select-pane -U\n", cfg.Keymap.PaneUp))
	b.WriteString(fmt.Sprintf("bind-key -n %s select-pane -R\n", cfg.Keymap.PaneRight))
	b.WriteString(fmt.Sprintf("bind-key -n %s command-prompt -I \"#(tmux list-windows -F '#I:#W' -t #{session_name} | tr '\\n' ' ')\" -p 'gmux cmd (new|1..9|splitv|splith|kill|dev|ops|logs|name)' 'run-shell \"\\\"%s\\\" palette -s #{session_name} \\\"%%%%\\\"\"'\n", cfg.Keymap.CommandPalette, exe))
	b.WriteString("bind-key -n M-v split-window -h\n")
	b.WriteString("bind-key -n M-s split-window -v\n")
	b.WriteString("bind-key -n C-g split-window -h\n")
	b.WriteString("bind-key -n C-/ display-message 'gmux keys: Alt+1..9 switch | Alt+n new | Alt+h/j/k/l panes | Alt+v/vsplit | Alt+s/hsplit | Ctrl+Space palette | Ctrl+q detach'\n")
	b.WriteString("bind-key -n C-_ display-message 'gmux keys: Alt+1..9 switch | Alt+n new | Alt+h/j/k/l panes | Alt+v/vsplit | Alt+s/hsplit | Ctrl+Space palette | Ctrl+q detach'\n")
	b.WriteString("bind-key -n C-q detach-client\n")
	b.WriteString("bind-key -n C-j detach-client\n")

	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return p, nil
}

func escapeSingle(in string) string {
	return strings.ReplaceAll(in, "'", "''")
}
