//go:build darwin

package hotkey

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation

#include <stdint.h>

extern int  startEventTap(int listenerID);
extern void stopEventTap(int listenerID);
*/
import "C"

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

// Modifier represents a macOS CGEvent modifier flag.
type Modifier uint64

const (
	ModShift  Modifier = 0x00020000 // kCGEventFlagMaskShift
	ModCtrl   Modifier = 0x00040000 // kCGEventFlagMaskControl
	ModOption Modifier = 0x00080000 // kCGEventFlagMaskAlternate
	ModCmd    Modifier = 0x00100000 // kCGEventFlagMaskCommand
)

// allModsMask covers the four modifier flags we match against.
const allModsMask = ModShift | ModCtrl | ModOption | ModCmd

// Key represents a macOS virtual key code.
type Key uint16

const (
	KeyA      Key = 0x00
	KeyS      Key = 0x01
	KeyD      Key = 0x02
	KeyF      Key = 0x03
	KeyH      Key = 0x04
	KeyG      Key = 0x05
	KeyZ      Key = 0x06
	KeyX      Key = 0x07
	KeyC      Key = 0x08
	KeyV      Key = 0x09
	KeyB      Key = 0x0B
	KeyQ      Key = 0x0C
	KeyW      Key = 0x0D
	KeyE      Key = 0x0E
	KeyR      Key = 0x0F
	KeyY      Key = 0x10
	KeyT      Key = 0x11
	Key1      Key = 0x12
	Key2      Key = 0x13
	Key3      Key = 0x14
	Key4      Key = 0x15
	Key6      Key = 0x16
	Key5      Key = 0x17
	Key9      Key = 0x19
	Key7      Key = 0x1A
	Key8      Key = 0x1C
	Key0      Key = 0x1D
	KeyO      Key = 0x1F
	KeyU      Key = 0x20
	KeyI      Key = 0x22
	KeyP      Key = 0x23
	KeyReturn Key = 0x24
	KeyL      Key = 0x25
	KeyJ      Key = 0x26
	KeyK      Key = 0x28
	KeyN      Key = 0x2D
	KeyM      Key = 0x2E
	KeyTab    Key = 0x30
	KeySpace  Key = 0x31
	KeyDelete Key = 0x33
	KeyEscape Key = 0x35
	KeyF17    Key = 0x40
	KeyF18    Key = 0x4F
	KeyF19    Key = 0x50
	KeyF20    Key = 0x5A
	KeyF5     Key = 0x60
	KeyF6     Key = 0x61
	KeyF7     Key = 0x62
	KeyF3     Key = 0x63
	KeyF8     Key = 0x64
	KeyF9     Key = 0x65
	KeyF11    Key = 0x67
	KeyF13    Key = 0x69
	KeyF16    Key = 0x6A
	KeyF14    Key = 0x6B
	KeyF10    Key = 0x6D
	KeyF12    Key = 0x6F
	KeyF15    Key = 0x71
	KeyF4     Key = 0x76
	KeyF2     Key = 0x78
	KeyF1     Key = 0x7A
	KeyLeft   Key = 0x7B
	KeyRight  Key = 0x7C
	KeyDown   Key = 0x7D
	KeyUp     Key = 0x7E
	KeyNone   Key = 0xFFFF // sentinel for modifier-only hotkeys
)

// modifierMap maps modifier name strings to Modifier values.
var modifierMap = map[string]Modifier{
	"OPTION": ModOption,
	"ALT":    ModOption,
	"CTRL":   ModCtrl,
	"SHIFT":  ModShift,
	"CMD":    ModCmd,
}

// keyMap maps key name strings to Key values.
var keyMap = map[string]Key{
	"SPACE":  KeySpace,
	"RETURN": KeyReturn,
	"ESCAPE": KeyEscape,
	"DELETE": KeyDelete,
	"TAB":    KeyTab,
	"LEFT":   KeyLeft,
	"RIGHT":  KeyRight,
	"UP":     KeyUp,
	"DOWN":   KeyDown,
	"F1":     KeyF1,
	"F2":     KeyF2,
	"F3":     KeyF3,
	"F4":     KeyF4,
	"F5":     KeyF5,
	"F6":     KeyF6,
	"F7":     KeyF7,
	"F8":     KeyF8,
	"F9":     KeyF9,
	"F10":    KeyF10,
	"F11":    KeyF11,
	"F12":    KeyF12,
	"F13":    KeyF13,
	"F14":    KeyF14,
	"F15":    KeyF15,
	"F16":    KeyF16,
	"F17":    KeyF17,
	"F18":    KeyF18,
	"F19":    KeyF19,
	"F20":    KeyF20,
	"A":      KeyA,
	"B":      KeyB,
	"C":      KeyC,
	"D":      KeyD,
	"E":      KeyE,
	"F":      KeyF,
	"G":      KeyG,
	"H":      KeyH,
	"I":      KeyI,
	"J":      KeyJ,
	"K":      KeyK,
	"L":      KeyL,
	"M":      KeyM,
	"N":      KeyN,
	"O":      KeyO,
	"P":      KeyP,
	"Q":      KeyQ,
	"R":      KeyR,
	"S":      KeyS,
	"T":      KeyT,
	"U":      KeyU,
	"V":      KeyV,
	"W":      KeyW,
	"X":      KeyX,
	"Y":      KeyY,
	"Z":      KeyZ,
	"0":      Key0,
	"1":      Key1,
	"2":      Key2,
	"3":      Key3,
	"4":      Key4,
	"5":      Key5,
	"6":      Key6,
	"7":      Key7,
	"8":      Key8,
	"9":      Key9,
}

// evdevKeyMap maps evdev-style KEY_ names to Key values for
// cross-platform config compatibility.
var evdevKeyMap = map[string]Key{
	"KEY_SPACE":  KeySpace,
	"KEY_ENTER":  KeyReturn,
	"KEY_ESC":    KeyEscape,
	"KEY_DELETE": KeyDelete,
	"KEY_TAB":    KeyTab,
	"KEY_LEFT":   KeyLeft,
	"KEY_RIGHT":  KeyRight,
	"KEY_UP":     KeyUp,
	"KEY_DOWN":   KeyDown,
	"KEY_F1":     KeyF1,
	"KEY_F2":     KeyF2,
	"KEY_F3":     KeyF3,
	"KEY_F4":     KeyF4,
	"KEY_F5":     KeyF5,
	"KEY_F6":     KeyF6,
	"KEY_F7":     KeyF7,
	"KEY_F8":     KeyF8,
	"KEY_F9":     KeyF9,
	"KEY_F10":    KeyF10,
	"KEY_F11":    KeyF11,
	"KEY_F12":    KeyF12,
	"KEY_F13":    KeyF13,
	"KEY_F14":    KeyF14,
	"KEY_F15":    KeyF15,
	"KEY_F16":    KeyF16,
	"KEY_F17":    KeyF17,
	"KEY_F18":    KeyF18,
	"KEY_F19":    KeyF19,
	"KEY_F20":    KeyF20,
	"KEY_A":      KeyA,
	"KEY_B":      KeyB,
	"KEY_C":      KeyC,
	"KEY_D":      KeyD,
	"KEY_E":      KeyE,
	"KEY_F":      KeyF,
	"KEY_G":      KeyG,
	"KEY_H":      KeyH,
	"KEY_I":      KeyI,
	"KEY_J":      KeyJ,
	"KEY_K":      KeyK,
	"KEY_L":      KeyL,
	"KEY_M":      KeyM,
	"KEY_N":      KeyN,
	"KEY_O":      KeyO,
	"KEY_P":      KeyP,
	"KEY_Q":      KeyQ,
	"KEY_R":      KeyR,
	"KEY_S":      KeyS,
	"KEY_T":      KeyT,
	"KEY_U":      KeyU,
	"KEY_V":      KeyV,
	"KEY_W":      KeyW,
	"KEY_X":      KeyX,
	"KEY_Y":      KeyY,
	"KEY_Z":      KeyZ,
	"KEY_0":      Key0,
	"KEY_1":      Key1,
	"KEY_2":      Key2,
	"KEY_3":      Key3,
	"KEY_4":      Key4,
	"KEY_5":      Key5,
	"KEY_6":      Key6,
	"KEY_7":      Key7,
	"KEY_8":      Key8,
	"KEY_9":      Key9,
}

// ParseHotkeyCombo parses a hotkey combo string like "Option+Space" or "Ctrl+F5"
// into modifiers, a key, and a display name. Also handles evdev-style "KEY_F12"
// for cross-platform config compatibility (mapped as bare key with no modifiers).
func ParseHotkeyCombo(combo string) ([]Modifier, Key, string, error) {
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
		return []Modifier{ModOption}, key, combo, nil
	}

	// Parse combo: "Option+Space", "Ctrl+Shift+F5", "Cmd+Option", etc.
	parts := strings.Split(combo, "+")
	if len(parts) < 2 {
		return nil, 0, "", fmt.Errorf("hotkey must be modifier+key or modifier+modifier (e.g. Option+Space, Cmd+Option), got: %s", combo)
	}

	// Check if the last part is a modifier (modifier-only combo like "Cmd+Option").
	lastPart := strings.TrimSpace(parts[len(parts)-1])
	if _, isMod := modifierMap[strings.ToUpper(lastPart)]; isMod {
		var mods []Modifier
		for _, part := range parts {
			part = strings.TrimSpace(part)
			mod, ok := modifierMap[strings.ToUpper(part)]
			if !ok {
				return nil, 0, "", fmt.Errorf("unknown modifier: %s (valid: Option, Alt, Ctrl, Shift, Cmd)", part)
			}
			mods = append(mods, mod)
		}
		return mods, KeyNone, combo, nil
	}

	// Last part is a key, everything before is a modifier.
	var mods []Modifier
	for _, part := range parts[:len(parts)-1] {
		part = strings.TrimSpace(part)
		mod, ok := modifierMap[strings.ToUpper(part)]
		if !ok {
			return nil, 0, "", fmt.Errorf("unknown modifier: %s (valid: Option, Alt, Ctrl, Shift, Cmd)", part)
		}
		mods = append(mods, mod)
	}

	key, ok := keyMap[strings.ToUpper(lastPart)]
	if !ok {
		return nil, 0, "", fmt.Errorf("unknown key: %s", lastPart)
	}

	return mods, key, combo, nil
}

// maxListenerID must match the fixed-size C arrays in cgeventtap_darwin.c.
const maxListenerID = 256

// Global registry for active listeners.
var (
	listenerMu     sync.Mutex
	listenerMap    = make(map[int]*darwinListener)
	nextListenerID int
	freedIDs       []int
)

// darwinListener implements the Listener interface using CGEventTap.
type darwinListener struct {
	mods    []Modifier
	modMask Modifier
	key     Key
	keyName string
	id      int
	onDown  func()
	onUp    func()
	active  bool // true while the hotkey is held down
	modOnly bool // true for modifier-only combos (e.g. Cmd+Option)
}

// NewListener creates a darwin hotkey Listener for the given modifiers, key, and display name.
func NewListener(mods []Modifier, key Key, keyName string) Listener {
	mask := Modifier(0)
	for _, m := range mods {
		mask |= m
	}
	return &darwinListener{mods: mods, modMask: mask, key: key, keyName: keyName, modOnly: key == KeyNone}
}

// allocListenerID returns a listener ID in [0, maxListenerID), reusing freed
// IDs when available. Must be called with listenerMu held.
func allocListenerID() (int, error) {
	if len(freedIDs) > 0 {
		id := freedIDs[len(freedIDs)-1]
		freedIDs = freedIDs[:len(freedIDs)-1]
		return id, nil
	}
	if nextListenerID >= maxListenerID {
		return 0, fmt.Errorf("hotkey listener limit reached (%d); cannot register more listeners", maxListenerID)
	}
	id := nextListenerID
	nextListenerID++
	return id, nil
}

// freeListenerID returns an ID to the free pool. Must be called with listenerMu held.
func freeListenerID(id int) {
	freedIDs = append(freedIDs, id)
}

// Start creates a CGEventTap and listens for hotkey events.
// It blocks until the context is cancelled or Stop is called.
func (l *darwinListener) Start(ctx context.Context, onDown func(), onUp func()) error {
	l.onDown = onDown
	l.onUp = onUp

	listenerMu.Lock()
	id, err := allocListenerID()
	if err != nil {
		listenerMu.Unlock()
		return err
	}
	l.id = id
	listenerMap[l.id] = l
	listenerMu.Unlock()

	// Watch for context cancellation and stop the event tap.
	go func() {
		<-ctx.Done()
		C.stopEventTap(C.int(l.id))
	}()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ret := C.startEventTap(C.int(l.id))

	listenerMu.Lock()
	delete(listenerMap, l.id)
	freeListenerID(l.id)
	listenerMu.Unlock()

	if ret != 0 {
		return fmt.Errorf("failed to create event tap for %s (grant Input Monitoring permission in System Settings > Privacy & Security > Input Monitoring)", l.keyName)
	}
	return ctx.Err()
}

// Stop stops the CGEventTap run loop, causing Start to return.
func (l *darwinListener) Stop() {
	C.stopEventTap(C.int(l.id))
}

// KeyName returns the configured hotkey combo string.
func (l *darwinListener) KeyName() string {
	return l.keyName
}

// CGEvent type constants.
const (
	cgEventKeyDown      = 10 // kCGEventKeyDown
	cgEventKeyUp        = 11 // kCGEventKeyUp
	cgEventFlagsChanged = 12 // kCGEventFlagsChanged
)

//export hotkeyEventCallback
func hotkeyEventCallback(listenerID C.int, eventType C.int, keycode C.int64_t, flags C.uint64_t) {
	listenerMu.Lock()
	l, ok := listenerMap[int(listenerID)]
	listenerMu.Unlock()
	if !ok {
		return
	}

	gotMods := Modifier(flags) & allModsMask

	if l.modOnly {
		// Modifier-only hotkey: react to flagsChanged events.
		if int(eventType) != cgEventFlagsChanged {
			return
		}
		if gotMods == l.modMask {
			if !l.active {
				l.active = true
				if l.onDown != nil {
					l.onDown()
				}
			}
		} else {
			if l.active {
				l.active = false
				if l.onUp != nil {
					l.onUp()
				}
			}
		}
		return
	}

	// Modifier+key hotkey: react to keyDown/keyUp events.
	if Key(keycode) != l.key {
		return
	}

	switch int(eventType) {
	case cgEventKeyDown:
		// Only fire onDown on the first press (ignore key repeats).
		if !l.active && gotMods == l.modMask {
			l.active = true
			if l.onDown != nil {
				l.onDown()
			}
		}
	case cgEventKeyUp:
		// Don't check modifiers on key-up — the user may have already
		// released the modifier key before releasing the main key.
		if l.active {
			l.active = false
			if l.onUp != nil {
				l.onUp()
			}
		}
	}
}
