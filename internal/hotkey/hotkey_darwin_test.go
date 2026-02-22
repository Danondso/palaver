//go:build darwin

package hotkey

import (
	"testing"

	"golang.design/x/hotkey"
)

func TestParseHotkeyCombo(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMods []hotkey.Modifier
		wantKey  hotkey.Key
		wantErr  bool
	}{
		{"option+space", "Option+Space", []hotkey.Modifier{hotkey.ModOption}, hotkey.KeySpace, false},
		{"ctrl+f5", "Ctrl+F5", []hotkey.Modifier{hotkey.ModCtrl}, hotkey.KeyF5, false},
		{"ctrl+shift+s", "Ctrl+Shift+S", []hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.KeyS, false},
		{"cmd+option+a", "Cmd+Option+A", []hotkey.Modifier{hotkey.ModCmd, hotkey.ModOption}, hotkey.KeyA, false},
		{"alt is option", "Alt+Space", []hotkey.Modifier{hotkey.ModOption}, hotkey.KeySpace, false},
		{"case insensitive", "option+space", []hotkey.Modifier{hotkey.ModOption}, hotkey.KeySpace, false},
		{"evdev key", "KEY_F12", []hotkey.Modifier{hotkey.ModOption}, hotkey.KeyF12, false},
		{"evdev space", "KEY_SPACE", []hotkey.Modifier{hotkey.ModOption}, hotkey.KeySpace, false},
		{"empty", "", nil, 0, true},
		{"no modifier", "Space", nil, 0, true},
		{"unknown modifier", "Super+Space", nil, 0, true},
		{"unknown key", "Option+Unknown", nil, 0, true},
		{"unknown evdev", "KEY_NONEXISTENT", nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mods, key, _, err := ParseHotkeyCombo(tt.input)
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
			if len(mods) != len(tt.wantMods) {
				t.Errorf("ParseHotkeyCombo(%q) mods = %v, want %v", tt.input, mods, tt.wantMods)
				return
			}
			for i := range mods {
				if mods[i] != tt.wantMods[i] {
					t.Errorf("ParseHotkeyCombo(%q) mod[%d] = %v, want %v", tt.input, i, mods[i], tt.wantMods[i])
				}
			}
			if key != tt.wantKey {
				t.Errorf("ParseHotkeyCombo(%q) key = %v, want %v", tt.input, key, tt.wantKey)
			}
		})
	}
}
