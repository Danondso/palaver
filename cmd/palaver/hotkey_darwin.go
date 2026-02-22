//go:build darwin

package main

import (
	"log"

	"github.com/gordonklaus/portaudio"

	"github.com/Danondso/palaver/internal/config"
	"github.com/Danondso/palaver/internal/hotkey"
)

func createListener(cfg *config.Config, dbg *log.Logger) (hotkey.Listener, error) {
	mods, key, keyName, err := hotkey.ParseHotkeyCombo(cfg.Hotkey.Key)
	if err != nil {
		return nil, err
	}
	dbg.Printf("hotkey: %s", keyName)

	return hotkey.NewListener(mods, key, keyName), nil
}

// initPortAudio initializes PortAudio. On macOS, no stderr suppression is needed
// since CoreAudio doesn't produce ALSA/JACK noise.
func initPortAudio() error {
	return portaudio.Initialize()
}
