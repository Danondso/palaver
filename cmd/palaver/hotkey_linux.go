//go:build linux

package main

import (
	"log"
	"os"
	"syscall"

	"github.com/gordonklaus/portaudio"

	"github.com/Danondso/palaver/internal/config"
	"github.com/Danondso/palaver/internal/hotkey"
)

func createListener(cfg *config.Config, dbg *log.Logger) (hotkey.Listener, error) {
	keyCode, err := hotkey.KeyCodeFromName(cfg.Hotkey.Key)
	if err != nil {
		return nil, err
	}
	dbg.Printf("hotkey: %s (code=%d)", cfg.Hotkey.Key, keyCode)

	dev, err := hotkey.FindKeyboard(cfg.Hotkey.Device)
	if err != nil {
		return nil, err
	}
	dbg.Printf("keyboard device: %s", dev.Path())

	return hotkey.NewListener(dev, keyCode, cfg.Hotkey.Key), nil
}

// initPortAudio suppresses ALSA/JACK noise during PortAudio initialization
// by temporarily redirecting stderr to /dev/null, then calls portaudio.Initialize().
func initPortAudio() error {
	stderrFd := int(os.Stderr.Fd())
	savedStderr, err := syscall.Dup(stderrFd)
	if err != nil {
		// If we can't dup stderr, just initialize without suppression
		return portaudio.Initialize()
	}
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		syscall.Close(savedStderr)
		return portaudio.Initialize()
	}
	syscall.Dup2(int(devNull.Fd()), stderrFd)
	devNull.Close()

	initErr := portaudio.Initialize()

	// Restore stderr
	syscall.Dup2(savedStderr, stderrFd)
	syscall.Close(savedStderr)

	return initErr
}
