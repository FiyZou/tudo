package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	appDirName     = ".tudo"
	dataDirName    = "data"
	configFileName = "config.json"
	databaseName   = "tudo.sqlite3"
)

type Config struct {
	AppDir       string `json:"-"`
	ConfigPath   string `json:"-"`
	DatabasePath string `json:"-"`

	UI     UIConfig     `json:"ui"`
	Colors ColorsConfig `json:"colors"`
}

type UIConfig struct {
	PageSize      int    `json:"page_size"`
	ShowCommands  bool   `json:"show_commands"`
	ShowPaths     bool   `json:"show_paths"`
	Prompt        string `json:"prompt"`
	InputPaddingY int    `json:"input_padding_y"`
	InputPaddingX int    `json:"input_padding_x"`
}

type ColorsConfig struct {
	HeaderForeground string `json:"header_foreground"`
	HeaderBackground string `json:"header_background"`

	MetaForegroundLight string `json:"meta_foreground_light"`
	MetaForegroundDark  string `json:"meta_foreground_dark"`

	ListForegroundLight string `json:"list_foreground_light"`
	ListForegroundDark  string `json:"list_foreground_dark"`

	DoneForegroundLight string `json:"done_foreground_light"`
	DoneForegroundDark  string `json:"done_foreground_dark"`

	EmptyForegroundLight string `json:"empty_foreground_light"`
	EmptyForegroundDark  string `json:"empty_foreground_dark"`

	ErrorForeground string `json:"error_foreground"`
	HelpForeground  string `json:"help_foreground"`
	HelpBackground  string `json:"help_background"`
	InputBackground string `json:"input_background"`
}

func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("find home directory: %w", err)
	}

	appDir := filepath.Join(home, appDirName)
	dataDir := filepath.Join(appDir, dataDirName)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data directory: %w", err)
	}

	cfg := Default()
	cfg.AppDir = appDir
	cfg.ConfigPath = filepath.Join(appDir, configFileName)
	cfg.DatabasePath = filepath.Join(dataDir, databaseName)

	if _, err := os.Stat(cfg.ConfigPath); err != nil {
		if !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("stat config file: %w", err)
		}
		if err := writeConfig(cfg.ConfigPath, cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}

	content, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}
	if err := json.Unmarshal(content, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file %s: %w", cfg.ConfigPath, err)
	}

	cfg.AppDir = appDir
	cfg.ConfigPath = filepath.Join(appDir, configFileName)
	cfg.DatabasePath = filepath.Join(dataDir, databaseName)
	cfg.normalize()

	return cfg, nil
}

func Default() Config {
	return Config{
		UI: UIConfig{
			PageSize:      12,
			ShowCommands:  true,
			ShowPaths:     true,
			Prompt:        "› ",
			InputPaddingY: 1,
			InputPaddingX: 1,
		},
		Colors: ColorsConfig{
			HeaderForeground: "229",
			HeaderBackground: "63",

			MetaForegroundLight: "240",
			MetaForegroundDark:  "244",

			ListForegroundLight: "232",
			ListForegroundDark:  "255",

			DoneForegroundLight: "244",
			DoneForegroundDark:  "241",

			EmptyForegroundLight: "240",
			EmptyForegroundDark:  "244",

			ErrorForeground: "203",
			HelpForeground:  "229",
			HelpBackground:  "238",
			InputBackground: "252",
		},
	}
}

func (c *Config) normalize() {
	defaults := Default()

	if c.UI.PageSize <= 0 {
		c.UI.PageSize = defaults.UI.PageSize
	}
	if c.UI.Prompt == "" {
		c.UI.Prompt = defaults.UI.Prompt
	}
	if c.UI.InputPaddingY < 0 {
		c.UI.InputPaddingY = defaults.UI.InputPaddingY
	}
	if c.UI.InputPaddingX < 0 {
		c.UI.InputPaddingX = defaults.UI.InputPaddingX
	}

	if c.Colors.HeaderForeground == "" {
		c.Colors.HeaderForeground = defaults.Colors.HeaderForeground
	}
	if c.Colors.HeaderBackground == "" {
		c.Colors.HeaderBackground = defaults.Colors.HeaderBackground
	}
	if c.Colors.MetaForegroundLight == "" {
		c.Colors.MetaForegroundLight = defaults.Colors.MetaForegroundLight
	}
	if c.Colors.MetaForegroundDark == "" {
		c.Colors.MetaForegroundDark = defaults.Colors.MetaForegroundDark
	}
	if c.Colors.ListForegroundLight == "" {
		c.Colors.ListForegroundLight = defaults.Colors.ListForegroundLight
	}
	if c.Colors.ListForegroundDark == "" {
		c.Colors.ListForegroundDark = defaults.Colors.ListForegroundDark
	}
	if c.Colors.DoneForegroundLight == "" {
		c.Colors.DoneForegroundLight = defaults.Colors.DoneForegroundLight
	}
	if c.Colors.DoneForegroundDark == "" {
		c.Colors.DoneForegroundDark = defaults.Colors.DoneForegroundDark
	}
	if c.Colors.EmptyForegroundLight == "" {
		c.Colors.EmptyForegroundLight = defaults.Colors.EmptyForegroundLight
	}
	if c.Colors.EmptyForegroundDark == "" {
		c.Colors.EmptyForegroundDark = defaults.Colors.EmptyForegroundDark
	}
	if c.Colors.ErrorForeground == "" {
		c.Colors.ErrorForeground = defaults.Colors.ErrorForeground
	}
	if c.Colors.HelpForeground == "" {
		c.Colors.HelpForeground = defaults.Colors.HelpForeground
	}
	if c.Colors.HelpBackground == "" {
		c.Colors.HelpBackground = defaults.Colors.HelpBackground
	}
	if c.Colors.InputBackground == "" {
		c.Colors.InputBackground = defaults.Colors.InputBackground
	}
}

func writeConfig(path string, cfg Config) error {
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode default config: %w", err)
	}
	content = append(content, '\n')

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}
