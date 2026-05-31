package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Keymap struct {
	NewWindow      string `yaml:"new_window"`
	CommandPalette string `yaml:"command_palette"`
	PaneLeft       string `yaml:"pane_left"`
	PaneDown       string `yaml:"pane_down"`
	PaneUp         string `yaml:"pane_up"`
	PaneRight      string `yaml:"pane_right"`
}

type Config struct {
	TmuxBin        string            `yaml:"tmux_bin"`
	DefaultSession string            `yaml:"default_session"`
	StatusTemplate string            `yaml:"status_template"`
	Keymap         Keymap            `yaml:"keymap"`
	Presets        map[string]string `yaml:"presets"`
}

func defaultConfig() Config {
	return Config{
		TmuxBin:        "tmux",
		DefaultSession: "dev",
		StatusTemplate: "Alt+n new | Alt+1..9 | Ctrl+q detach",
		Keymap: Keymap{
			NewWindow:      "M-n",
			CommandPalette: "C-Space",
			PaneLeft:       "M-h",
			PaneDown:       "M-j",
			PaneUp:         "M-k",
			PaneRight:      "M-l",
		},
		Presets: map[string]string{
			"dev":  "code",
			"ops":  "htop",
			"logs": "tail -f /var/log/system.log",
		},
	}
}

func Path() string {
	if p := os.Getenv("GOMUX_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gmux", "config.yaml")
}

func Load() (Config, error) {
	cfg := defaultConfig()
	p := Path()
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			if err := WriteDefault(p, cfg); err != nil {
				return Config{}, err
			}
			return cfg, nil
		}
		return Config{}, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	mergeDefaults(&cfg)
	return cfg, nil
}

func WriteDefault(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func mergeDefaults(cfg *Config) {
	d := defaultConfig()
	if cfg.TmuxBin == "" {
		cfg.TmuxBin = d.TmuxBin
	}
	if cfg.DefaultSession == "" {
		cfg.DefaultSession = d.DefaultSession
	}
	if cfg.StatusTemplate == "" {
		cfg.StatusTemplate = d.StatusTemplate
	}
	if cfg.Keymap.NewWindow == "" {
		cfg.Keymap.NewWindow = d.Keymap.NewWindow
	}
	if cfg.Keymap.CommandPalette == "" {
		cfg.Keymap.CommandPalette = d.Keymap.CommandPalette
	}
	if cfg.Keymap.PaneLeft == "" {
		cfg.Keymap.PaneLeft = d.Keymap.PaneLeft
	}
	if cfg.Keymap.PaneDown == "" {
		cfg.Keymap.PaneDown = d.Keymap.PaneDown
	}
	if cfg.Keymap.PaneUp == "" {
		cfg.Keymap.PaneUp = d.Keymap.PaneUp
	}
	if cfg.Keymap.PaneRight == "" {
		cfg.Keymap.PaneRight = d.Keymap.PaneRight
	}
	if len(cfg.Presets) == 0 {
		cfg.Presets = d.Presets
	}
}
