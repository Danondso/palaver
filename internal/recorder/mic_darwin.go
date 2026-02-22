//go:build darwin

package recorder

import "github.com/gordonklaus/portaudio"

// MicName returns a human-readable name for the default input device.
// On macOS, this simply returns the PortAudio device name.
func MicName() string {
	dev, err := portaudio.DefaultInputDevice()
	if err != nil || dev == nil {
		return ""
	}
	return dev.Name
}
