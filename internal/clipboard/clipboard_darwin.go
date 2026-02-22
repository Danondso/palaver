//go:build darwin

package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PasteText inserts text into the currently focused application.
// mode "clipboard" (default on macOS) uses pbcopy + Cmd+V via osascript.
// mode "type" uses osascript keystroke for direct typing.
func PasteText(text string, delayMs int, mode string) error {
	if delayMs > 0 {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	if mode == "type" {
		return typeAppleScript(text)
	}

	return pasteClipboard(text)
}

// pasteClipboard writes text to the macOS clipboard via pbcopy,
// then simulates Cmd+V via osascript.
func pasteClipboard(text string) error {
	// Write to clipboard via pbcopy
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy: %w", err)
	}

	// Simulate Cmd+V
	script := `tell application "System Events" to keystroke "v" using command down`
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return fmt.Errorf("osascript Cmd+V: %w (grant Accessibility permissions in System Settings > Privacy & Security)", err)
	}

	// Clear clipboard after a short delay (best-effort).
	// Error is intentionally ignored: clearing is a courtesy, and failure
	// (e.g. pbcopy not found) should not prevent a successful paste.
	time.Sleep(100 * time.Millisecond)
	clearCmd := exec.Command("pbcopy")
	clearCmd.Stdin = strings.NewReader("")
	_ = clearCmd.Run()

	return nil
}

// typeAppleScript types text directly using osascript keystroke.
func typeAppleScript(text string) error {
	escaped := escapeAppleScript(text)
	script := fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escaped)
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return fmt.Errorf("osascript keystroke: %w (grant Accessibility permissions in System Settings > Privacy & Security)", err)
	}
	return nil
}

// escapeAppleScript escapes a string for use inside AppleScript double quotes.
// Handles backslashes, double quotes, and control characters that could break
// out of the string literal or execute unintended AppleScript.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\b", "")
	return s
}
