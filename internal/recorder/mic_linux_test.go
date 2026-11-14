//go:build linux

package recorder

import "testing"

const sampleSources = `Source #57
	State: SUSPENDED
	Name: alsa_output.pci-0000_7a_00.1.hdmi-stereo-extra2.monitor
	Description: Monitor of Built-in Audio Digital Stereo (HDMI)
Source #205
	State: SUSPENDED
	Name: alsa_output.usb-Universal_Audio_Volt_276-00.analog-stereo.monitor
	Description: Monitor of Volt 276 Analog Stereo
Source #206
	State: SUSPENDED
	Name: alsa_input.usb-Universal_Audio_Volt_276-00.analog-stereo
	Description: Volt 276 Analog Stereo
`

func TestClassifyDefaultSource(t *testing.T) {
	tests := []struct {
		name          string
		defaultSource string
		list          string
		wantAvailable bool
		wantName      string
	}{
		{
			name:          "real present mic",
			defaultSource: "alsa_input.usb-Universal_Audio_Volt_276-00.analog-stereo",
			list:          sampleSources,
			wantAvailable: true,
			wantName:      "Volt 276 Analog Stereo",
		},
		{
			name:          "default is a monitor (mic unplugged, fell back)",
			defaultSource: "alsa_output.usb-Universal_Audio_Volt_276-00.analog-stereo.monitor",
			list:          sampleSources,
			wantAvailable: false,
		},
		{
			name:          "default points at a source that is gone",
			defaultSource: "alsa_input.usb-Some_Unplugged_Mic-00.analog-stereo",
			list:          sampleSources,
			wantAvailable: false,
		},
		{
			name:          "empty default source",
			defaultSource: "",
			list:          sampleSources,
			wantAvailable: false,
		},
		{
			name:          "present but described as a monitor",
			defaultSource: "alsa_output.usb-Universal_Audio_Volt_276-00.analog-stereo.monitor",
			list:          sampleSources,
			wantAvailable: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAvail, gotName := classifyDefaultSource(tt.defaultSource, tt.list)
			if gotAvail != tt.wantAvailable {
				t.Errorf("available = %v, want %v", gotAvail, tt.wantAvailable)
			}
			if gotAvail && gotName != tt.wantName {
				t.Errorf("name = %q, want %q", gotName, tt.wantName)
			}
		})
	}
}

func TestParseSourceDescription(t *testing.T) {
	desc, present := parseSourceDescription(sampleSources, "alsa_input.usb-Universal_Audio_Volt_276-00.analog-stereo")
	if !present {
		t.Fatal("expected source to be present")
	}
	if desc != "Volt 276 Analog Stereo" {
		t.Errorf("desc = %q, want %q", desc, "Volt 276 Analog Stereo")
	}

	if _, present := parseSourceDescription(sampleSources, "alsa_input.nonexistent"); present {
		t.Error("expected missing source to report not present")
	}
}
