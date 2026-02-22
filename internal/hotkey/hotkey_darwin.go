//go:build darwin

package hotkey

import (
	"context"
	"fmt"
	"strings"

	"golang.design/x/hotkey"
)

// modifierMap maps modifier name strings to hotkey.Modifier values.
var modifierMap = map[string]hotkey.Modifier{
	"OPTION": hotkey.ModOption,
	"ALT":    hotkey.ModOption,
	"CTRL":   hotkey.ModCtrl,
	"SHIFT":  hotkey.ModShift,
	"CMD":    hotkey.ModCmd,
}

// keyMap maps key name strings to hotkey.Key values.
var keyMap = map[string]hotkey.Key{
	"SPACE":  hotkey.KeySpace,
	"RETURN": hotkey.KeyReturn,
	"ESCAPE": hotkey.KeyEscape,
	"DELETE": hotkey.KeyDelete,
	"TAB":    hotkey.KeyTab,
	"LEFT":   hotkey.KeyLeft,
	"RIGHT":  hotkey.KeyRight,
	"UP":     hotkey.KeyUp,
	"DOWN":   hotkey.KeyDown,
	"F1":     hotkey.KeyF1,
	"F2":     hotkey.KeyF2,
	"F3":     hotkey.KeyF3,
	"F4":     hotkey.KeyF4,
	"F5":     hotkey.KeyF5,
	"F6":     hotkey.KeyF6,
	"F7":     hotkey.KeyF7,
	"F8":     hotkey.KeyF8,
	"F9":     hotkey.KeyF9,
	"F10":    hotkey.KeyF10,
	"F11":    hotkey.KeyF11,
	"F12":    hotkey.KeyF12,
	"F13":    hotkey.KeyF13,
	"F14":    hotkey.KeyF14,
	"F15":    hotkey.KeyF15,
	"F16":    hotkey.KeyF16,
	"F17":    hotkey.KeyF17,
	"F18":    hotkey.KeyF18,
	"F19":    hotkey.KeyF19,
	"F20":    hotkey.KeyF20,
	"A":      hotkey.KeyA,
	"B":      hotkey.KeyB,
	"C":      hotkey.KeyC,
	"D":      hotkey.KeyD,
	"E":      hotkey.KeyE,
	"F":      hotkey.KeyF,
	"G":      hotkey.KeyG,
	"H":      hotkey.KeyH,
	"I":      hotkey.KeyI,
	"J":      hotkey.KeyJ,
	"K":      hotkey.KeyK,
	"L":      hotkey.KeyL,
	"M":      hotkey.KeyM,
	"N":      hotkey.KeyN,
	"O":      hotkey.KeyO,
	"P":      hotkey.KeyP,
	"Q":      hotkey.KeyQ,
	"R":      hotkey.KeyR,
	"S":      hotkey.KeyS,
	"T":      hotkey.KeyT,
	"U":      hotkey.KeyU,
	"V":      hotkey.KeyV,
	"W":      hotkey.KeyW,
	"X":      hotkey.KeyX,
	"Y":      hotkey.KeyY,
	"Z":      hotkey.KeyZ,
	"0":      hotkey.Key0,
	"1":      hotkey.Key1,
	"2":      hotkey.Key2,
	"3":      hotkey.Key3,
	"4":      hotkey.Key4,
	"5":      hotkey.Key5,
	"6":      hotkey.Key6,
	"7":      hotkey.Key7,
	"8":      hotkey.Key8,
	"9":      hotkey.Key9,
}

// evdevKeyMap maps evdev-style KEY_ names to hotkey.Key values for
// cross-platform config compatibility.
var evdevKeyMap = map[string]hotkey.Key{
	"KEY_SPACE":  hotkey.KeySpace,
	"KEY_ENTER":  hotkey.KeyReturn,
	"KEY_ESC":    hotkey.KeyEscape,
	"KEY_DELETE": hotkey.KeyDelete,
	"KEY_TAB":    hotkey.KeyTab,
	"KEY_LEFT":   hotkey.KeyLeft,
	"KEY_RIGHT":  hotkey.KeyRight,
	"KEY_UP":     hotkey.KeyUp,
	"KEY_DOWN":   hotkey.KeyDown,
	"KEY_F1":     hotkey.KeyF1,
	"KEY_F2":     hotkey.KeyF2,
	"KEY_F3":     hotkey.KeyF3,
	"KEY_F4":     hotkey.KeyF4,
	"KEY_F5":     hotkey.KeyF5,
	"KEY_F6":     hotkey.KeyF6,
	"KEY_F7":     hotkey.KeyF7,
	"KEY_F8":     hotkey.KeyF8,
	"KEY_F9":     hotkey.KeyF9,
	"KEY_F10":    hotkey.KeyF10,
	"KEY_F11":    hotkey.KeyF11,
	"KEY_F12":    hotkey.KeyF12,
	"KEY_F13":    hotkey.KeyF13,
	"KEY_F14":    hotkey.KeyF14,
	"KEY_F15":    hotkey.KeyF15,
	"KEY_F16":    hotkey.KeyF16,
	"KEY_F17":    hotkey.KeyF17,
	"KEY_F18":    hotkey.KeyF18,
	"KEY_F19":    hotkey.KeyF19,
	"KEY_F20":    hotkey.KeyF20,
	"KEY_A":      hotkey.KeyA,
	"KEY_B":      hotkey.KeyB,
	"KEY_C":      hotkey.KeyC,
	"KEY_D":      hotkey.KeyD,
	"KEY_E":      hotkey.KeyE,
	"KEY_F":      hotkey.KeyF,
	"KEY_G":      hotkey.KeyG,
	"KEY_H":      hotkey.KeyH,
	"KEY_I":      hotkey.KeyI,
	"KEY_J":      hotkey.KeyJ,
	"KEY_K":      hotkey.KeyK,
	"KEY_L":      hotkey.KeyL,
	"KEY_M":      hotkey.KeyM,
	"KEY_N":      hotkey.KeyN,
	"KEY_O":      hotkey.KeyO,
	"KEY_P":      hotkey.KeyP,
	"KEY_Q":      hotkey.KeyQ,
	"KEY_R":      hotkey.KeyR,
	"KEY_S":      hotkey.KeyS,
	"KEY_T":      hotkey.KeyT,
	"KEY_U":      hotkey.KeyU,
	"KEY_V":      hotkey.KeyV,
	"KEY_W":      hotkey.KeyW,
	"KEY_X":      hotkey.KeyX,
	"KEY_Y":      hotkey.KeyY,
	"KEY_Z":      hotkey.KeyZ,
	"KEY_0":      hotkey.Key0,
	"KEY_1":      hotkey.Key1,
	"KEY_2":      hotkey.Key2,
	"KEY_3":      hotkey.Key3,
	"KEY_4":      hotkey.Key4,
	"KEY_5":      hotkey.Key5,
	"KEY_6":      hotkey.Key6,
	"KEY_7":      hotkey.Key7,
	"KEY_8":      hotkey.Key8,
	"KEY_9":      hotkey.Key9,
}

// ParseHotkeyCombo parses a hotkey combo string like "Option+Space" or "Ctrl+F5"
// into modifiers, a key, and a display name. Also handles evdev-style "KEY_F12"
// for cross-platform config compatibility (mapped as bare key with no modifiers).
func ParseHotkeyCombo(combo string) ([]hotkey.Modifier, hotkey.Key, string, error) {
	combo = strings.TrimSpace(combo)
	if combo == "" {
		return nil, 0, "", fmt.Errorf("empty hotkey combo")
	}

	upper := strings.ToUpper(combo)

	// Handle evdev-style KEY_ names (bare key, no modifiers — use Option as default modifier)
	if strings.HasPrefix(upper, "KEY_") {
		key, ok := evdevKeyMap[upper]
		if !ok {
			return nil, 0, "", fmt.Errorf("unknown evdev key: %s (on macOS, use modifier+key combos like Option+Space)", combo)
		}
		return []hotkey.Modifier{hotkey.ModOption}, key, combo, nil
	}

	// Parse modifier+key combo: "Option+Space", "Ctrl+Shift+F5", etc.
	parts := strings.Split(combo, "+")
	if len(parts) < 2 {
		return nil, 0, "", fmt.Errorf("hotkey must be modifier+key (e.g. Option+Space), got: %s", combo)
	}

	var mods []hotkey.Modifier
	for _, part := range parts[:len(parts)-1] {
		part = strings.TrimSpace(part)
		mod, ok := modifierMap[strings.ToUpper(part)]
		if !ok {
			return nil, 0, "", fmt.Errorf("unknown modifier: %s (valid: Option, Alt, Ctrl, Shift, Cmd)", part)
		}
		mods = append(mods, mod)
	}

	keyStr := strings.TrimSpace(parts[len(parts)-1])
	key, ok := keyMap[strings.ToUpper(keyStr)]
	if !ok {
		return nil, 0, "", fmt.Errorf("unknown key: %s", keyStr)
	}

	return mods, key, combo, nil
}

// darwinListener implements the Listener interface using golang.design/x/hotkey.
type darwinListener struct {
	mods    []hotkey.Modifier
	key     hotkey.Key
	keyName string
	hk      *hotkey.Hotkey
}

// NewListener creates a darwin hotkey Listener for the given modifiers, key, and display name.
func NewListener(mods []hotkey.Modifier, key hotkey.Key, keyName string) Listener {
	return &darwinListener{mods: mods, key: key, keyName: keyName}
}

// Start registers the hotkey and listens for press/release events.
// It blocks until the context is cancelled.
func (l *darwinListener) Start(ctx context.Context, onDown func(), onUp func()) error {
	l.hk = hotkey.New(l.mods, l.key)
	if err := l.hk.Register(); err != nil {
		return fmt.Errorf("register hotkey %s: %w (grant Accessibility permissions in System Settings > Privacy & Security)", l.keyName, err)
	}

	for {
		select {
		case <-ctx.Done():
			l.hk.Unregister()
			return ctx.Err()
		case <-l.hk.Keydown():
			if onDown != nil {
				onDown()
			}
		case <-l.hk.Keyup():
			if onUp != nil {
				onUp()
			}
		}
	}
}

// Stop unregisters the hotkey.
func (l *darwinListener) Stop() {
	if l.hk != nil {
		l.hk.Unregister()
	}
}

// KeyName returns the configured hotkey combo string.
func (l *darwinListener) KeyName() string {
	return l.keyName
}
