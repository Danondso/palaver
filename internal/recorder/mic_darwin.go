//go:build darwin

package recorder

import "github.com/gordonklaus/portaudio"

// probeDevice reports whether a usable microphone is present and its name.
// On macOS, CoreAudio updates the default input device when hardware changes,
// so a PortAudio re-initialization (reinit) refreshes enumeration before the
// query.
func probeDevice(reinit func() error) MicStatus {
	if reinit != nil {
		if err := reinit(); err != nil {
			return MicStatus{}
		}
	}
	dev, err := portaudio.DefaultInputDevice()
	if err != nil || dev == nil || dev.MaxInputChannels <= 0 {
		return MicStatus{}
	}
	return MicStatus{Available: true, Name: dev.Name}
}

// MicName returns a human-readable name for the default input device, or "" if
// none is available.
func MicName() string {
	return probeDevice(nil).Name
}
