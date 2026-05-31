package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"gmux/internal/config"
)

func Palette(cfg config.Config, args []string) error {
	session := ""
	raw := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "-s" && i+1 < len(args) {
			session = args[i+1]
			i++
			continue
		}
		if raw == "" {
			raw = args[i]
		}
	}
	if session == "" {
		return fmt.Errorf("palette requires -s <session>")
	}
	if raw == "" {
		return nil
	}

	s := session
	if !strings.HasPrefix(s, "gmux-") {
		s = "gmux-" + s
	}
	cmd := strings.ToLower(strings.TrimSpace(raw))

	// Fast aliases: numbers switch windows directly.
	if n, err := strconv.Atoi(cmd); err == nil && n >= 1 && n <= 9 {
		return runTmux(cfg.TmuxBin, "select-window", "-t", fmt.Sprintf("%s:%d", s, n))
	}

	switch cmd {
	case "new", "n":
		return runTmux(cfg.TmuxBin, "new-window", "-t", s)
	case "next":
		return runTmux(cfg.TmuxBin, "next-window", "-t", s)
	case "prev", "previous":
		return runTmux(cfg.TmuxBin, "previous-window", "-t", s)
	case "splitv", "vsplit":
		return runTmux(cfg.TmuxBin, "split-window", "-h", "-t", s)
	case "splith", "hsplit":
		return runTmux(cfg.TmuxBin, "split-window", "-v", "-t", s)
	case "kill":
		return runTmux(cfg.TmuxBin, "kill-window", "-t", s)
	case "dev", "ops", "logs":
		if p, ok := cfg.Presets[cmd]; ok && strings.TrimSpace(p) != "" {
			return runTmux(cfg.TmuxBin, "send-keys", "-t", s+":1", p, "C-m")
		}
		return nil
	default:
		// Friendly fallback: treat input as rename title.
		return runTmux(cfg.TmuxBin, "rename-window", "-t", s, raw)
	}
}

func runTmux(tmuxBin string, args ...string) error {
	c := exec.Command(tmuxBin, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}
