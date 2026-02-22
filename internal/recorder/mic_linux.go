//go:build linux

package recorder

import (
	"os/exec"
	"strings"

	"github.com/gordonklaus/portaudio"
)

// MicName returns a human-readable name for the default input device.
// It tries pactl (PulseAudio/PipeWire) first for a descriptive name,
// then falls back to the PortAudio device name.
func MicName() string {
	// Try pactl for a descriptive name (works with PulseAudio and PipeWire)
	if name := micNameFromPactl(); name != "" {
		return name
	}
	dev, err := portaudio.DefaultInputDevice()
	if err != nil || dev == nil {
		return ""
	}
	return dev.Name
}

func micNameFromPactl() string {
	// Get the default source name
	out, err := exec.Command("pactl", "get-default-source").Output()
	if err != nil {
		return ""
	}
	sourceName := strings.TrimSpace(string(out))
	if sourceName == "" {
		return ""
	}

	// Get its description
	out, err = exec.Command("pactl", "list", "sources").Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(out), "\n")
	inSource := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Name: ") {
			inSource = strings.TrimPrefix(trimmed, "Name: ") == sourceName
		}
		if inSource && strings.HasPrefix(trimmed, "Description: ") {
			desc := strings.TrimPrefix(trimmed, "Description: ")
			// Skip monitor sources (they capture output, not mic input)
			if strings.HasPrefix(desc, "Monitor of ") {
				return ""
			}
			return desc
		}
	}
	return ""
}
