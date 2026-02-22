//go:build linux

package hotkey

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	evdev "github.com/holoplot/go-evdev"
)

// keyNameMap maps evdev key name strings to their numeric codes.
var keyNameMap = map[string]evdev.EvCode{
	"KEY_ESC":        1,
	"KEY_1":          2,
	"KEY_2":          3,
	"KEY_3":          4,
	"KEY_4":          5,
	"KEY_5":          6,
	"KEY_6":          7,
	"KEY_7":          8,
	"KEY_8":          9,
	"KEY_9":          10,
	"KEY_0":          11,
	"KEY_MINUS":      12,
	"KEY_EQUAL":      13,
	"KEY_BACKSPACE":  14,
	"KEY_TAB":        15,
	"KEY_Q":          16,
	"KEY_W":          17,
	"KEY_E":          18,
	"KEY_R":          19,
	"KEY_T":          20,
	"KEY_Y":          21,
	"KEY_U":          22,
	"KEY_I":          23,
	"KEY_O":          24,
	"KEY_P":          25,
	"KEY_LEFTBRACE":  26,
	"KEY_RIGHTBRACE": 27,
	"KEY_ENTER":      28,
	"KEY_LEFTCTRL":   29,
	"KEY_A":          30,
	"KEY_S":          31,
	"KEY_D":          32,
	"KEY_F":          33,
	"KEY_G":          34,
	"KEY_H":          35,
	"KEY_J":          36,
	"KEY_K":          37,
	"KEY_L":          38,
	"KEY_SEMICOLON":  39,
	"KEY_APOSTROPHE": 40,
	"KEY_GRAVE":      41,
	"KEY_LEFTSHIFT":  42,
	"KEY_BACKSLASH":  43,
	"KEY_Z":          44,
	"KEY_X":          45,
	"KEY_C":          46,
	"KEY_V":          47,
	"KEY_B":          48,
	"KEY_N":          49,
	"KEY_M":          50,
	"KEY_COMMA":      51,
	"KEY_DOT":        52,
	"KEY_SLASH":      53,
	"KEY_RIGHTSHIFT": 54,
	"KEY_KPASTERISK": 55,
	"KEY_LEFTALT":    56,
	"KEY_SPACE":      57,
	"KEY_CAPSLOCK":   58,
	"KEY_F1":         59,
	"KEY_F2":         60,
	"KEY_F3":         61,
	"KEY_F4":         62,
	"KEY_F5":         63,
	"KEY_F6":         64,
	"KEY_F7":         65,
	"KEY_F8":         66,
	"KEY_F9":         67,
	"KEY_F10":        68,
	"KEY_NUMLOCK":    69,
	"KEY_SCROLLLOCK": 70,
	"KEY_F11":        87,
	"KEY_F12":        88,
	"KEY_RIGHTCTRL":  97,
	"KEY_RIGHTALT":   100,
	"KEY_HOME":       102,
	"KEY_UP":         103,
	"KEY_PAGEUP":     104,
	"KEY_LEFT":       105,
	"KEY_RIGHT":      106,
	"KEY_END":        107,
	"KEY_DOWN":       108,
	"KEY_PAGEDOWN":   109,
	"KEY_INSERT":     110,
	"KEY_DELETE":     111,
	"KEY_PAUSE":      119,
	"KEY_LEFTMETA":   125,
	"KEY_RIGHTMETA":  126,
	"KEY_F13":        183,
	"KEY_F14":        184,
	"KEY_F15":        185,
	"KEY_F16":        186,
	"KEY_F17":        187,
	"KEY_F18":        188,
	"KEY_F19":        189,
	"KEY_F20":        190,
	"KEY_F21":        191,
	"KEY_F22":        192,
	"KEY_F23":        193,
	"KEY_F24":        194,
}

// KeyCodeFromName maps an evdev key name string to its numeric key code.
func KeyCodeFromName(name string) (evdev.EvCode, error) {
	upper := strings.ToUpper(strings.TrimSpace(name))
	code, ok := keyNameMap[upper]
	if !ok {
		return 0, fmt.Errorf("unknown key name: %s", name)
	}
	return code, nil
}

// FindKeyboard opens a specific device path, or auto-detects a keyboard
// by scanning /dev/input/event* for devices that support letter keys
// (KEY_A through KEY_Z), distinguishing real keyboards from power buttons
// and other devices that only have EV_KEY capability.
func FindKeyboard(devicePath string) (*evdev.InputDevice, error) {
	if devicePath != "" {
		dev, err := evdev.Open(devicePath)
		if err != nil {
			return nil, fmt.Errorf("open device %s: %w", devicePath, err)
		}
		return dev, nil
	}

	matches, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, fmt.Errorf("glob /dev/input/event*: %w", err)
	}

	// Sort numerically so event7 comes before event10
	sort.Slice(matches, func(i, j int) bool {
		ni, _ := strconv.Atoi(strings.TrimPrefix(matches[i], "/dev/input/event"))
		nj, _ := strconv.Atoi(strings.TrimPrefix(matches[j], "/dev/input/event"))
		return ni < nj
	})

	for _, path := range matches {
		dev, err := evdev.Open(path)
		if err != nil {
			continue
		}

		if isKeyboard(dev) {
			return dev, nil
		}
		_ = dev.Close()
	}

	return nil, fmt.Errorf("no keyboard device found in /dev/input/event*")
}

// isKeyboard returns true if the device supports letter keys (KEY_A..KEY_Z)
// and is not a mouse (no EV_REL capability), identifying it as a real keyboard
// rather than a power button, mouse, or other device.
func isKeyboard(dev *evdev.InputDevice) bool {
	// Reject devices with relative axes (mice, trackpads)
	for _, evType := range dev.CapableTypes() {
		if evType == evdev.EV_REL {
			return false
		}
	}

	keys := dev.CapableEvents(evdev.EV_KEY)
	hasA := false
	hasZ := false
	for _, code := range keys {
		if code == 30 { // KEY_A
			hasA = true
		}
		if code == 44 { // KEY_Z
			hasZ = true
		}
	}
	return hasA && hasZ
}

// linuxListener listens for global hotkey press/release events via evdev.
type linuxListener struct {
	dev     *evdev.InputDevice
	keyCode evdev.EvCode
	keyName string
	mu      sync.Mutex
	closed  bool
}

// NewListener creates a Listener for the given evdev device, key code, and key name.
func NewListener(dev *evdev.InputDevice, keyCode evdev.EvCode, keyName string) Listener {
	return &linuxListener{dev: dev, keyCode: keyCode, keyName: keyName}
}

// Start blocks and reads evdev events, calling onDown on key press and
// onUp on key release for the configured key code. It returns when the
// context is cancelled or the device is closed.
func (l *linuxListener) Start(ctx context.Context, onDown func(), onUp func()) error {
	errCh := make(chan error, 1)

	go func() {
		for {
			ev, err := l.dev.ReadOne()
			if err != nil {
				l.mu.Lock()
				closed := l.closed
				l.mu.Unlock()
				if closed {
					errCh <- nil
					return
				}
				if os.IsNotExist(err) || strings.Contains(err.Error(), "file already closed") || strings.Contains(err.Error(), "bad file descriptor") {
					errCh <- nil
					return
				}
				errCh <- fmt.Errorf("read event: %w", err)
				return
			}

			if ev.Type != evdev.EV_KEY || ev.Code != l.keyCode {
				continue
			}
			switch ev.Value {
			case 1: // key down
				if onDown != nil {
					onDown()
				}
			case 0: // key up
				if onUp != nil {
					onUp()
				}
				// value 2 = key repeat, ignored
			}
		}
	}()

	select {
	case <-ctx.Done():
		l.Stop()
		<-errCh
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// Stop closes the evdev device and stops the listener.
func (l *linuxListener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.closed {
		l.closed = true
		_ = l.dev.Close()
	}
}

// KeyName returns the configured key name string.
func (l *linuxListener) KeyName() string {
	return l.keyName
}
