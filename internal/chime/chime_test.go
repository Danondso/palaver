package chime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWithDefaults(t *testing.T) {
	p, err := New("", "", true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.startData) == 0 {
		t.Error("expected non-empty start data from embedded default")
	}
	if len(p.stopData) == 0 {
		t.Error("expected non-empty stop data from embedded default")
	}
	if !p.enabled {
		t.Error("expected enabled")
	}
}

func TestNewDisabled(t *testing.T) {
	p, err := New("", "", false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.enabled {
		t.Error("expected disabled")
	}
	// PlayStart/PlayStop should be no-ops when disabled
	p.PlayStart()
	p.PlayStop()
}

func TestNewWithCustomPaths(t *testing.T) {
	dir := t.TempDir()
	startPath := filepath.Join(dir, "custom_start.wav")
	stopPath := filepath.Join(dir, "custom_stop.wav")

	// Write the embedded defaults as custom files for testing
	if err := os.WriteFile(startPath, defaultStartWav, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stopPath, defaultStopWav, 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := New(startPath, stopPath, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.startData) == 0 {
		t.Error("expected non-empty start data from custom path")
	}
	if len(p.stopData) == 0 {
		t.Error("expected non-empty stop data from custom path")
	}
}

func TestNewWithBadPath(t *testing.T) {
	_, err := New("/nonexistent/path/start.wav", "", true, nil)
	if err == nil {
		t.Error("expected error for nonexistent start path")
	}

	_, err = New("", "/nonexistent/path/stop.wav", true, nil)
	if err == nil {
		t.Error("expected error for nonexistent stop path")
	}
}

func TestEmbeddedChimesNotEmpty(t *testing.T) {
	if len(defaultStartWav) < 44 {
		t.Errorf("embedded start.wav too small: %d bytes", len(defaultStartWav))
	}
	if len(defaultStopWav) < 44 {
		t.Errorf("embedded stop.wav too small: %d bytes", len(defaultStopWav))
	}
}
