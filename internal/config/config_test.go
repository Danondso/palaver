package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultValues(t *testing.T) {
	cfg := Default()

	if cfg.Hotkey.Key != "KEY_RIGHTCTRL" {
		t.Errorf("expected hotkey KEY_RIGHTCTRL, got %s", cfg.Hotkey.Key)
	}
	if cfg.Hotkey.Device != "" {
		t.Errorf("expected empty device, got %s", cfg.Hotkey.Device)
	}
	if cfg.Audio.TargetSampleRate != 16000 {
		t.Errorf("expected sample rate 16000, got %d", cfg.Audio.TargetSampleRate)
	}
	if cfg.Audio.MaxDurationSec != 60 {
		t.Errorf("expected max duration 60, got %d", cfg.Audio.MaxDurationSec)
	}
	if !cfg.Audio.ChimeEnabled {
		t.Error("expected chime enabled by default")
	}
	if cfg.Transcription.Provider != "openai" {
		t.Errorf("expected provider openai, got %s", cfg.Transcription.Provider)
	}
	if cfg.Transcription.BaseURL != "http://localhost:5092" {
		t.Errorf("expected base URL http://localhost:5092, got %s", cfg.Transcription.BaseURL)
	}
	if cfg.Transcription.Model != "whisper-1" {
		t.Errorf("expected model whisper-1, got %s", cfg.Transcription.Model)
	}
	if cfg.Transcription.TimeoutSec != 30 {
		t.Errorf("expected timeout 30, got %d", cfg.Transcription.TimeoutSec)
	}
	if cfg.Paste.DelayMs != 50 {
		t.Errorf("expected paste delay 50, got %d", cfg.Paste.DelayMs)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if cfg.Hotkey.Key != "KEY_RIGHTCTRL" {
		t.Errorf("expected default hotkey, got %s", cfg.Hotkey.Key)
	}
}

func TestLoadOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[hotkey]
key = "KEY_F12"
device = "/dev/input/event5"

[audio]
target_sample_rate = 48000
max_duration_sec = 60
chime_enabled = false

[transcription]
provider = "command"
base_url = "http://localhost:8080"
model = "whisper-1"
timeout_sec = 10
command = "whisper-cpp -f {input}"

[paste]
delay_ms = 100
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Hotkey.Key != "KEY_F12" {
		t.Errorf("expected KEY_F12, got %s", cfg.Hotkey.Key)
	}
	if cfg.Hotkey.Device != "/dev/input/event5" {
		t.Errorf("expected /dev/input/event5, got %s", cfg.Hotkey.Device)
	}
	if cfg.Audio.TargetSampleRate != 48000 {
		t.Errorf("expected 48000, got %d", cfg.Audio.TargetSampleRate)
	}
	if cfg.Audio.MaxDurationSec != 60 {
		t.Errorf("expected 60, got %d", cfg.Audio.MaxDurationSec)
	}
	if cfg.Audio.ChimeEnabled {
		t.Error("expected chime disabled")
	}
	if cfg.Transcription.Provider != "command" {
		t.Errorf("expected command, got %s", cfg.Transcription.Provider)
	}
	if cfg.Transcription.BaseURL != "http://localhost:8080" {
		t.Errorf("expected http://localhost:8080, got %s", cfg.Transcription.BaseURL)
	}
	if cfg.Transcription.Model != "whisper-1" {
		t.Errorf("expected whisper-1, got %s", cfg.Transcription.Model)
	}
	if cfg.Transcription.TimeoutSec != 10 {
		t.Errorf("expected 10, got %d", cfg.Transcription.TimeoutSec)
	}
	if cfg.Transcription.Command != "whisper-cpp -f {input}" {
		t.Errorf("expected whisper-cpp -f {input}, got %s", cfg.Transcription.Command)
	}
	if cfg.Paste.DelayMs != 100 {
		t.Errorf("expected 100, got %d", cfg.Paste.DelayMs)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := Default()
	cfg.Theme = "gruvbox"
	cfg.Transcription.Model = "large-v3"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}

	if loaded.Theme != "gruvbox" {
		t.Errorf("expected theme gruvbox, got %s", loaded.Theme)
	}
	if loaded.Transcription.Model != "large-v3" {
		t.Errorf("expected model large-v3, got %s", loaded.Transcription.Model)
	}
	if loaded.Hotkey.Key != "KEY_RIGHTCTRL" {
		t.Errorf("expected default hotkey preserved, got %s", loaded.Hotkey.Key)
	}
	if loaded.Audio.TargetSampleRate != 16000 {
		t.Errorf("expected default sample rate preserved, got %d", loaded.Audio.TargetSampleRate)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "config.toml")

	cfg := Default()
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save failed to create nested dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist at %s: %v", path, err)
	}
}

func TestLoadPartialOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[hotkey]
key = "KEY_F5"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Hotkey.Key != "KEY_F5" {
		t.Errorf("expected KEY_F5, got %s", cfg.Hotkey.Key)
	}
	// Non-overridden values should remain defaults
	if cfg.Transcription.BaseURL != "http://localhost:5092" {
		t.Errorf("expected default base URL, got %s", cfg.Transcription.BaseURL)
	}
	if cfg.Paste.DelayMs != 50 {
		t.Errorf("expected default paste delay 50, got %d", cfg.Paste.DelayMs)
	}
}
