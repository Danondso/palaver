package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// HotkeyConfig holds hotkey-related settings.
type HotkeyConfig struct {
	Key    string `toml:"key"`
	Device string `toml:"device"`
}

// AudioConfig holds audio capture settings.
type AudioConfig struct {
	TargetSampleRate int    `toml:"target_sample_rate"`
	MaxDurationSec   int    `toml:"max_duration_sec"`
	ChimeStart       string `toml:"chime_start"`
	ChimeStop        string `toml:"chime_stop"`
	ChimeEnabled     bool   `toml:"chime_enabled"`
}

// TranscriptionConfig holds transcription provider settings.
type TranscriptionConfig struct {
	Provider   string `toml:"provider"`
	BaseURL    string `toml:"base_url"`
	Model      string `toml:"model"`
	TimeoutSec int    `toml:"timeout_sec"`
	Command    string `toml:"command"`
}

// PasteConfig holds clipboard paste settings.
type PasteConfig struct {
	DelayMs int `toml:"delay_ms"`
}

// Config is the top-level configuration.
type Config struct {
	Hotkey        HotkeyConfig        `toml:"hotkey"`
	Audio         AudioConfig         `toml:"audio"`
	Transcription TranscriptionConfig `toml:"transcription"`
	Paste         PasteConfig         `toml:"paste"`
}

// Default returns a Config populated with all default values.
func Default() *Config {
	return &Config{
		Hotkey: HotkeyConfig{
			Key:    "KEY_RIGHTCTRL",
			Device: "",
		},
		Audio: AudioConfig{
			TargetSampleRate: 16000,
			MaxDurationSec:   30,
			ChimeStart:       "",
			ChimeStop:        "",
			ChimeEnabled:     true,
		},
		Transcription: TranscriptionConfig{
			Provider:   "openai",
			BaseURL:    "http://localhost:5092",
			Model:      "default",
			TimeoutSec: 30,
			Command:    "",
		},
		Paste: PasteConfig{
			DelayMs: 50,
		},
	}
}

// DefaultPath returns the default config file path (~/.config/palaver/config.toml).
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "palaver", "config.toml")
}

// Load reads the TOML config from path. If the file does not exist,
// it returns the default config without error.
func Load(path string) (*Config, error) {
	cfg := Default()

	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	_, err = toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
