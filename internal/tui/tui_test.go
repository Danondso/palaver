package tui

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"

	"github.com/Danondso/palaver/internal/config"
)

// mockTranscriber implements transcriber.Transcriber for testing.
type mockTranscriber struct {
	result string
	err    error
}

func (m *mockTranscriber) Transcribe(_ context.Context, _ []byte) (string, error) {
	return m.result, m.err
}

type mockLevelSampler struct {
	level float64
}

func (m *mockLevelSampler) AudioLevel() float64 {
	return m.level
}

func newTestModel() Model {
	cfg := config.Default()
	return NewModel(cfg, &mockTranscriber{result: "test text"}, nil, nil, nil, log.New(io.Discard, "", 0), false)
}

func TestInitialState(t *testing.T) {
	m := newTestModel()
	if m.State != StateIdle {
		t.Errorf("expected StateIdle, got %d", m.State)
	}
	if m.LastTranscript != "" {
		t.Error("expected empty transcript")
	}
}

func TestRecordingStartedTransition(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(RecordingStartedMsg{})
	model := updated.(Model)
	if model.State != StateRecording {
		t.Errorf("expected StateRecording, got %d", model.State)
	}
}

func TestRecordingStoppedTransition(t *testing.T) {
	m := newTestModel()
	m.State = StateRecording
	updated, cmd := m.Update(RecordingStoppedMsg{WavData: []byte("wav")})
	model := updated.(Model)
	if model.State != StateTranscribing {
		t.Errorf("expected StateTranscribing, got %d", model.State)
	}
	if cmd == nil {
		t.Error("expected transcription command")
	}
}

func TestTranscriptionResultTransition(t *testing.T) {
	m := newTestModel()
	m.State = StateTranscribing
	updated, _ := m.Update(TranscriptionResultMsg{Text: "hello world"})
	model := updated.(Model)
	if model.State != StatePasting {
		t.Errorf("expected StatePasting, got %d", model.State)
	}
	if model.LastTranscript != "hello world" {
		t.Errorf("expected 'hello world', got %q", model.LastTranscript)
	}
}

func TestTranscriptionErrorTransition(t *testing.T) {
	m := newTestModel()
	m.State = StateTranscribing
	updated, cmd := m.Update(TranscriptionErrorMsg{Err: fmt.Errorf("connection refused")})
	model := updated.(Model)
	if model.State != StateError {
		t.Errorf("expected StateError, got %d", model.State)
	}
	if model.LastError != "connection refused" {
		t.Errorf("expected 'connection refused', got %q", model.LastError)
	}
	if cmd == nil {
		t.Error("expected error timeout command")
	}
}

func TestErrorTimeoutTransition(t *testing.T) {
	m := newTestModel()
	m.State = StateError
	m.LastError = "some error"
	updated, _ := m.Update(errorTimeoutMsg{})
	model := updated.(Model)
	if model.State != StateIdle {
		t.Errorf("expected StateIdle, got %d", model.State)
	}
	if model.LastError != "" {
		t.Errorf("expected empty error, got %q", model.LastError)
	}
}

func TestViewContainsTitle(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !contains(view, "PALAVER") {
		t.Error("expected view to contain 'PALAVER'")
	}
}

func TestViewShowsIdleBadge(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !contains(view, "Idle") {
		t.Error("expected view to contain 'Idle'")
	}
}

func TestDebugLogMsgAddsEntry(t *testing.T) {
	m := newTestModel()
	entry := DebugEntry{Time: "11:00:00", Category: "hotkey", Message: "hello"}
	updated, _ := m.Update(DebugLogMsg{Entry: entry})
	model := updated.(Model)
	if len(model.DebugEntries) != 1 {
		t.Fatalf("expected 1 debug entry, got %d", len(model.DebugEntries))
	}
	if model.DebugEntries[0].Message != "hello" {
		t.Errorf("expected 'hello', got %q", model.DebugEntries[0].Message)
	}
}

func TestDebugLogTruncatesToMax(t *testing.T) {
	m := newTestModel()
	for i := 0; i < maxDebugLines+10; i++ {
		entry := DebugEntry{Time: "11:00:00", Category: "debug", Message: fmt.Sprintf("line %d", i)}
		updated, _ := m.Update(DebugLogMsg{Entry: entry})
		m = updated.(Model)
	}
	if len(m.DebugEntries) != maxDebugLines {
		t.Errorf("expected %d debug entries, got %d", maxDebugLines, len(m.DebugEntries))
	}
	if m.DebugEntries[0].Message != "line 10" {
		t.Errorf("expected oldest message to be 'line 10', got %q", m.DebugEntries[0].Message)
	}
}

func TestViewShowsDebugPanel(t *testing.T) {
	m := newTestModel()
	entry := DebugEntry{Time: "11:00:00", Category: "hotkey", Message: "test message"}
	updated, _ := m.Update(DebugLogMsg{Entry: entry})
	model := updated.(Model)
	view := model.View()
	if !contains(view, "Debug") {
		t.Error("expected view to contain 'Debug' panel title")
	}
	if !contains(view, "test message") {
		t.Error("expected view to contain debug message")
	}
}

func TestViewHidesDebugPanelWhenEmpty(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if contains(view, "Debug") {
		t.Error("expected view to NOT contain 'Debug' panel when no debug lines")
	}
}

func TestParseLineStructured(t *testing.T) {
	entry := parseLine("[DEBUG] 11:27:53.777842 hotkey up: KEY_RIGHTCTRL")
	if entry.Time != "11:27:53.777842" {
		t.Errorf("expected time '11:27:53.777842', got %q", entry.Time)
	}
	if entry.Category != "hotkey" {
		t.Errorf("expected category 'hotkey', got %q", entry.Category)
	}
	if entry.Message != "hotkey up: KEY_RIGHTCTRL" {
		t.Errorf("expected message 'hotkey up: KEY_RIGHTCTRL', got %q", entry.Message)
	}
}

func TestViewShowsTranscript(t *testing.T) {
	m := newTestModel()
	m.LastTranscript = "hello world"
	view := m.View()
	if !contains(view, "hello world") {
		t.Error("expected view to contain transcript")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAudioLevelTickUpdatesLevel(t *testing.T) {
	m := newTestModel()
	m.State = StateRecording
	m.Recorder = &mockLevelSampler{level: 0.42}
	updated, cmd := m.Update(audioLevelTickMsg{})
	model := updated.(Model)
	if model.AudioLevel != 0.42 {
		t.Errorf("expected AudioLevel 0.42, got %f", model.AudioLevel)
	}
	if cmd == nil {
		t.Error("expected another tick command while recording")
	}
}

func TestAudioLevelTickResetsWhenNotRecording(t *testing.T) {
	m := newTestModel()
	m.State = StateIdle
	m.AudioLevel = 0.5
	m.Recorder = &mockLevelSampler{level: 0.42}
	updated, cmd := m.Update(audioLevelTickMsg{})
	model := updated.(Model)
	if model.AudioLevel != 0 {
		t.Errorf("expected AudioLevel 0, got %f", model.AudioLevel)
	}
	if cmd != nil {
		t.Error("expected no tick command when not recording")
	}
}

func TestStatusCheckMsgUpdatesModel(t *testing.T) {
	m := newTestModel()
	updated, cmd := m.Update(StatusCheckMsg{MicDetected: true, BackendOnline: false})
	model := updated.(Model)
	if !model.MicDetected {
		t.Error("expected MicDetected to be true")
	}
	if model.BackendOnline {
		t.Error("expected BackendOnline to be false")
	}
	if !model.statusChecked {
		t.Error("expected statusChecked to be true")
	}
	if cmd == nil {
		t.Error("expected recheck schedule command")
	}
}

func TestVisualizerAppearsWhenRecording(t *testing.T) {
	m := newTestModel()
	m.State = StateRecording
	m.AudioLevel = 0.5
	view := m.View()
	if !contains(view, "█") {
		t.Error("expected view to contain visualizer bar when recording")
	}
	if !contains(view, "Mic") {
		t.Error("expected view to contain 'Mic' label for visualizer")
	}
}

func TestVisualizerHiddenWhenIdle(t *testing.T) {
	m := newTestModel()
	m.State = StateIdle
	view := m.View()
	if contains(view, "░░░░░░░░░░░░░░░░░░░░") {
		t.Error("expected view to NOT contain full visualizer bar when idle")
	}
}

func TestStatusBarAppearsInView(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !contains(view, "Mic:") {
		t.Error("expected view to contain 'Mic:' status indicator")
	}
	if !contains(view, "Backend:") {
		t.Error("expected view to contain 'Backend:' status indicator")
	}
}

func TestRecordingStartedReturnsTickCmd(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(RecordingStartedMsg{})
	if cmd == nil {
		t.Error("expected audio level tick command on recording start")
	}
}

func TestRecordingStoppedResetsAudioLevel(t *testing.T) {
	m := newTestModel()
	m.State = StateRecording
	m.AudioLevel = 0.7
	updated, _ := m.Update(RecordingStoppedMsg{WavData: []byte("wav")})
	model := updated.(Model)
	if model.AudioLevel != 0 {
		t.Errorf("expected AudioLevel 0 after stop, got %f", model.AudioLevel)
	}
}
