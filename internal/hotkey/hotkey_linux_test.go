//go:build linux

package hotkey

import (
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

func TestKeyCodeFromName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected evdev.EvCode
		wantErr  bool
	}{
		{"right ctrl", "KEY_RIGHTCTRL", 97, false},
		{"f12", "KEY_F12", 88, false},
		{"space", "KEY_SPACE", 57, false},
		{"left alt", "KEY_LEFTALT", 56, false},
		{"case insensitive", "key_rightctrl", 97, false},
		{"with whitespace", "  KEY_F12  ", 88, false},
		{"unknown key", "KEY_NONEXISTENT", 0, true},
		{"empty string", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := KeyCodeFromName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
				return
			}
			if code != tt.expected {
				t.Errorf("KeyCodeFromName(%q) = %d, want %d", tt.input, code, tt.expected)
			}
		})
	}
}
