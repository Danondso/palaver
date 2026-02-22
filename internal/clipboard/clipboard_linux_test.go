//go:build linux

package clipboard

import (
	"os"
	"testing"
)

func TestIsWayland(t *testing.T) {
	// Just verify the function runs without panic
	_ = isWayland()
}

func TestIsWaylandDetection(t *testing.T) {
	orig := os.Getenv("WAYLAND_DISPLAY")
	defer func() { _ = os.Setenv("WAYLAND_DISPLAY", orig) }()

	if err := os.Setenv("WAYLAND_DISPLAY", "wayland-0"); err != nil {
		t.Fatal(err)
	}
	if !isWayland() {
		t.Error("expected isWayland()=true when WAYLAND_DISPLAY is set")
	}

	if err := os.Unsetenv("WAYLAND_DISPLAY"); err != nil {
		t.Fatal(err)
	}
	if isWayland() {
		t.Error("expected isWayland()=false when WAYLAND_DISPLAY is unset")
	}
}

func TestPasteTextRequiresDisplay(t *testing.T) {
	t.Log("clipboard.PasteText requires a display server for full testing")
}
