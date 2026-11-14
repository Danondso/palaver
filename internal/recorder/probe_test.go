package recorder

import (
	"errors"
	"testing"

	"github.com/gordonklaus/portaudio"
)

// TestProbeMicSkipsReinitWhileRecording verifies ProbeMic never re-initializes
// PortAudio while a capture stream is live, since that would race the read
// loop. It must report the last known status instead.
func TestProbeMicSkipsReinitWhileRecording(t *testing.T) {
	r := &Recorder{recording: true, available: true, micName: "Cached Mic"}
	called := false
	r.reinit = func() error { called = true; return nil }

	st := r.ProbeMic()

	if called {
		t.Error("reinit must not run while recording")
	}
	if !st.Available || st.Name != "Cached Mic" {
		t.Errorf("ProbeMic while recording = %+v, want cached status", st)
	}
}

// TestProbeMicSkipsReinitWithOpenStream verifies the same guard applies during
// the stop window, when recording has been cleared but the stream is not yet
// closed.
func TestProbeMicSkipsReinitWithOpenStream(t *testing.T) {
	r := &Recorder{stream: new(portaudio.Stream), available: true, micName: "Cached Mic"}
	called := false
	r.reinit = func() error { called = true; return nil }

	st := r.ProbeMic()

	if called {
		t.Error("reinit must not run while a stream is open")
	}
	if !st.Available || st.Name != "Cached Mic" {
		t.Errorf("ProbeMic with open stream = %+v, want cached status", st)
	}
}

// TestProbeMicReinitFailureReportsUnavailable verifies that when the device
// list cannot be refreshed, the mic is reported as unavailable rather than
// falling back to a stale "available" state.
func TestProbeMicReinitFailureReportsUnavailable(t *testing.T) {
	r := &Recorder{available: true, micName: "Stale Mic"}
	r.reinit = func() error { return errors.New("reinit failed") }

	st := r.ProbeMic()

	if st.Available || st.Name != "" {
		t.Errorf("ProbeMic after reinit failure = %+v, want unavailable", st)
	}
	if r.available || r.micName != "" {
		t.Errorf("cached status after reinit failure = {%v %q}, want cleared", r.available, r.micName)
	}
}
