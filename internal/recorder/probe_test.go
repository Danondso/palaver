package recorder

import (
	"testing"

	"github.com/gordonklaus/portaudio"
)

// withProbeDevice swaps the platform probe for a stub for the duration of a test.
func withProbeDevice(t *testing.T, fn func(reinit func() error) MicStatus) {
	t.Helper()
	orig := probeDeviceFn
	probeDeviceFn = fn
	t.Cleanup(func() { probeDeviceFn = orig })
}

// TestProbeMicReturnsCachedWhileRecording verifies ProbeMic never runs the
// device probe while a capture stream is live (which could re-initialize
// PortAudio and race the read loop); it reports the last known status instead.
func TestProbeMicReturnsCachedWhileRecording(t *testing.T) {
	probed := false
	withProbeDevice(t, func(func() error) MicStatus {
		probed = true
		return MicStatus{}
	})

	r := &Recorder{recording: true, available: true, micName: "Cached Mic"}
	st := r.ProbeMic()

	if probed {
		t.Error("probe must not run while recording")
	}
	if !st.Available || st.Name != "Cached Mic" {
		t.Errorf("ProbeMic while recording = %+v, want cached status", st)
	}
}

// TestProbeMicReturnsCachedWithOpenStream covers the stop window, when recording
// is cleared but the stream is not yet closed.
func TestProbeMicReturnsCachedWithOpenStream(t *testing.T) {
	probed := false
	withProbeDevice(t, func(func() error) MicStatus {
		probed = true
		return MicStatus{}
	})

	r := &Recorder{stream: new(portaudio.Stream), available: true, micName: "Cached Mic"}
	st := r.ProbeMic()

	if probed {
		t.Error("probe must not run while a stream is open")
	}
	if !st.Available || st.Name != "Cached Mic" {
		t.Errorf("ProbeMic with open stream = %+v, want cached status", st)
	}
}

// TestProbeMicUpdatesCacheFromProbe verifies ProbeMic reflects the probe result
// and updates the cached status used while recording.
func TestProbeMicUpdatesCacheFromProbe(t *testing.T) {
	withProbeDevice(t, func(func() error) MicStatus {
		return MicStatus{Available: true, Name: "USB Mic"}
	})
	r := &Recorder{available: false}
	if st := r.ProbeMic(); !st.Available || st.Name != "USB Mic" {
		t.Errorf("ProbeMic = %+v, want available USB Mic", st)
	}
	if !r.available || r.micName != "USB Mic" {
		t.Errorf("cached status = {%v %q}, want available USB Mic", r.available, r.micName)
	}
}

// TestProbeMicUnavailableClearsCache verifies a missing mic clears the cached
// status rather than leaving a stale "available" state.
func TestProbeMicUnavailableClearsCache(t *testing.T) {
	withProbeDevice(t, func(func() error) MicStatus {
		return MicStatus{}
	})
	r := &Recorder{available: true, micName: "Stale Mic"}
	if st := r.ProbeMic(); st.Available || st.Name != "" {
		t.Errorf("ProbeMic = %+v, want unavailable", st)
	}
	if r.available || r.micName != "" {
		t.Errorf("cached status = {%v %q}, want cleared", r.available, r.micName)
	}
}
