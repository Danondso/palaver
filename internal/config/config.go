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
	Provider      string `toml:"provider"`
	BaseURL       string `toml:"base_url"`
	Model         string `toml:"model"`
	TimeoutSec    int    `toml:"timeout_sec"`
	Command       string `toml:"command"`
	TLSSkipVerify bool   `toml:"tls_skip_verify"`
}

// PasteConfig holds clipboard paste settings.
type PasteConfig struct {
	DelayMs int    `toml:"delay_ms"`
	Mode    string `toml:"mode"` // "type" (direct typing) or "clipboard" (Ctrl+V)
}

// ServerConfig holds managed backend server settings.
type ServerConfig struct {
	AutoStart bool   `toml:"auto_start"`
	DataDir   string `toml:"data_dir"`
	Port      int    `toml:"port"`
}

// Config is the top-level configuration.
type Config struct {
	Theme         string              `toml:"theme"`
	Hotkey        HotkeyConfig        `toml:"hotkey"`
	Audio         AudioConfig         `toml:"audio"`
	Transcription TranscriptionConfig `toml:"transcription"`
	Paste         PasteConfig         `toml:"paste"`
	Server        ServerConfig        `toml:"server"`
}

// Default returns a Config populated with all default values.
func Default() *Config {
	return &Config{
		Theme: "synthwave",
		Hotkey: HotkeyConfig{
			Key:    defaultHotkeyKey,
			Device: "",
		},
		Audio: AudioConfig{
			TargetSampleRate: 16000,
			MaxDurationSec:   60,
			ChimeStart:       "",
			ChimeStop:        "",
			ChimeEnabled:     true,
		},
		Transcription: TranscriptionConfig{
			Provider:   "openai",
			BaseURL:    "http://localhost:5092",
			Model:      "whisper-1",
			TimeoutSec: 30,
			Command:    "",
		},
		Paste: PasteConfig{
			DelayMs: 50,
			Mode:    defaultPasteMode,
		},
		Server: ServerConfig{
			AutoStart: true,
			DataDir:   "",
			Port:      5092,
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

// DefaultDataDir returns the default data directory (~/.local/share/palaver).
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "palaver")
}

// Save writes the config as TOML to the given path, creating parent
// directories if needed. The write is atomic: data is written to a
// temporary file and renamed into place so a crash mid-write cannot
// corrupt the existing config.
func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".palaver-config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if err := toml.NewEncoder(tmp).Encode(cfg); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
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
