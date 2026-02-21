package clipboard

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	atclip "github.com/atotto/clipboard"
)

// isWayland returns true if the session is running under Wayland.
func isWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

// PasteText inserts text into the currently focused application.
// On Wayland it uses wtype to type text directly (avoids clipboard mismatch
// between X11 and Wayland). On X11 it writes to clipboard and simulates Ctrl+V.
func PasteText(text string, delayMs int) error {
	if delayMs > 0 {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	if isWayland() {
		return typeWayland(text)
	}
	return pasteX11(text)
}

// ensureYdotoold starts ydotoold in the background if it's not already running.
// Called once at init time.
func ensureYdotoold() {
	// Check if ydotoold is already running
	if err := exec.Command("pgrep", "-x", "ydotoold").Run(); err == nil {
		return // already running
	}
	if _, err := exec.LookPath("ydotoold"); err != nil {
		return // not installed
	}
	cmd := exec.Command("ydotoold")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return
	}
	// Give it a moment to initialize
	time.Sleep(200 * time.Millisecond)
}

func typeWayland(text string) error {
	// Use wl-copy to set the Wayland clipboard, then ydotool to press Ctrl+V.
	// ydotool works via /dev/uinput (kernel-level) which works on all compositors.
	if _, err := exec.LookPath("wl-copy"); err != nil {
		return fmt.Errorf("wl-copy not found: %w (install with: apt install wl-clipboard)", err)
	}
	if _, err := exec.LookPath("ydotool"); err != nil {
		return fmt.Errorf("ydotool not found: %w (install with: apt install ydotool)", err)
	}

	ensureYdotoold()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "wl-copy", "--", text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wl-copy: %w", err)
	}
	cmd = exec.CommandContext(ctx, "ydotool", "key", "--delay", "0", "ctrl+v")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ydotool key ctrl+v: %w", err)
	}

	// Clear clipboard after paste (best-effort)
	time.Sleep(100 * time.Millisecond)
	exec.CommandContext(ctx, "wl-copy", "--clear").Run()

	return nil
}

func pasteX11(text string) error {
	if _, err := exec.LookPath("xdotool"); err != nil {
		return fmt.Errorf("xdotool not found: %w (install with: apt install xdotool)", err)
	}
	if err := atclip.WriteAll(text); err != nil {
		return fmt.Errorf("write to clipboard: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "xdotool", "key", "ctrl+v")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xdotool paste: %w", err)
	}

	// Clear clipboard after paste (best-effort)
	time.Sleep(100 * time.Millisecond)
	atclip.WriteAll("")

	return nil
}
