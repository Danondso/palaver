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
	defer os.Setenv("WAYLAND_DISPLAY", orig)

	os.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if !isWayland() {
		t.Error("expected isWayland()=true when WAYLAND_DISPLAY is set")
	}

	os.Unsetenv("WAYLAND_DISPLAY")
	if isWayland() {
		t.Error("expected isWayland()=false when WAYLAND_DISPLAY is unset")
	}
}

func TestPasteTextRequiresDisplay(t *testing.T) {
	t.Log("clipboard.PasteText requires a display server for full testing")
}
