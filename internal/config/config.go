package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	APIProvider   string `toml:"api_provider"`
	APIKey        string `toml:"api_key"`
	FastModel     string `toml:"fast_model"`
	SmartModel    string `toml:"smart_model"`
	AutoGitignore bool   `toml:"auto_gitignore"`
	SQLitePath    string `toml:"sqlite_path"`
}

func Default() *Config {
	return &Config{
		APIProvider:   "openrouter",
		FastModel:     "google/gemini-2.0-flash-exp",
		SmartModel:    "anthropic/claude-sonnet-4-20250514",
		AutoGitignore: true,
		SQLitePath:    filepath.Join(configDir(), "omnix.db"),
	}
}

func Path() string {
	return filepath.Join(configDir(), "config.toml")
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "omnix")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "omnix")
}

func Load() (*Config, error) {
	p := Path()
	cfg := Default()

	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil, fmt.Errorf("config not found at %s", p)
	}

	if _, err := toml.DecodeFile(p, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if env := os.Getenv("OMNIX_API_KEY"); env != "" {
		cfg.APIKey = env
	}

	if cfg.SQLitePath == "" {
		cfg.SQLitePath = filepath.Join(configDir(), "omnix.db")
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	f, err := os.OpenFile(Path(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	return enc.Encode(cfg)
}
