//go:build linux

package recorder

import (
	"os/exec"
	"strings"

	"github.com/gordonklaus/portaudio"
)

// probeDevice reports whether a usable microphone is present and its name.
//
// On Linux it trusts PulseAudio/PipeWire (via pactl) rather than PortAudio:
// when routed through the Pulse/PipeWire ALSA bridge, PortAudio always reports
// a persistent "default" input device even after the hardware is unplugged, so
// it can never signal a disconnect. pactl, by contrast, drops the source when
// the device goes away and the default falls back to a monitor. PortAudio
// enumeration is used only when no PulseAudio/PipeWire server is reachable
// (raw ALSA).
// reinit, when non-nil, re-initializes PortAudio to refresh its device list.
// It is only used on the raw-ALSA fallback path; the pactl path reads live
// PulseAudio/PipeWire state and needs no refresh.
func probeDevice(reinit func() error) MicStatus {
	if available, name, ok := probePulseSource(); ok {
		if !available {
			return MicStatus{}
		}
		return MicStatus{Available: true, Name: name}
	}

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

// probePulseSource queries PulseAudio/PipeWire for the default input source.
// ok is false when no server is reachable (pactl missing or erroring), so the
// caller can fall back to PortAudio. When ok is true, available reports whether
// the default source is a real (non-monitor) capture device that is still
// present, and name is its description.
func probePulseSource() (available bool, name string, ok bool) {
	out, err := exec.Command("pactl", "get-default-source").Output()
	if err != nil {
		return false, "", false
	}
	defaultSource := strings.TrimSpace(string(out))

	listOut, err := exec.Command("pactl", "list", "sources").Output()
	if err != nil {
		// Server is reachable but the source list is unavailable; fall back to
		// the default-source name alone (a monitor means no real mic).
		if defaultSource == "" || strings.HasSuffix(defaultSource, ".monitor") {
			return false, "", true
		}
		return true, defaultSource, true
	}

	avail, desc := classifyDefaultSource(defaultSource, string(listOut))
	return avail, desc, true
}

// classifyDefaultSource decides whether the default PulseAudio/PipeWire source
// is a real, present capture device, returning its description when so. It is
// pure so it can be tested against captured pactl output.
func classifyDefaultSource(defaultSource, listSources string) (available bool, name string) {
	if defaultSource == "" || strings.HasSuffix(defaultSource, ".monitor") {
		return false, ""
	}
	desc, present := parseSourceDescription(listSources, defaultSource)
	if !present || desc == "" || strings.HasPrefix(desc, "Monitor of ") {
		return false, ""
	}
	return true, desc
}

// parseSourceDescription returns the Description of the named source from the
// output of `pactl list sources`, and whether that source was found.
func parseSourceDescription(listSources, source string) (desc string, present bool) {
	inSource := false
	for _, line := range strings.Split(listSources, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Name: ") {
			inSource = strings.TrimPrefix(trimmed, "Name: ") == source
			continue
		}
		if inSource && strings.HasPrefix(trimmed, "Description: ") {
			return strings.TrimPrefix(trimmed, "Description: "), true
		}
	}
	return "", false
}
