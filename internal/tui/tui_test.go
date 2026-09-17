package tui

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Danondso/palaver/internal/config"
	"github.com/Danondso/palaver/internal/postprocess"
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

type mockContinuousRecorder struct {
	level      float64
	started    bool
	continuous bool
	stopped    bool
	takeWAV    []byte
	takeErr    error
	stopWAV    []byte
	stopErr    error
	startErr   error
}

func (m *mockContinuousRecorder) AudioLevel() float64 { return m.level }

func (m *mockContinuousRecorder) Start() error {
	m.started = true
	return m.startErr
}

func (m *mockContinuousRecorder) StartContinuous() error {
	m.continuous = true
	m.started = true
	return m.startErr
}

func (m *mockContinuousRecorder) TakeWAV() ([]byte, error) {
	return m.takeWAV, m.takeErr
}

func (m *mockContinuousRecorder) Stop() ([]byte, bool, error) {
	m.stopped = true
	return m.stopWAV, false, m.stopErr
}

type mockPostProcessor struct {
	result string
	err    error
}

func (m *mockPostProcessor) Rewrite(_ context.Context, text string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.result != "" {
		return m.result, nil
	}
	return text, nil
}

func newTestModel() Model {
	cfg := config.Default()
	return NewModel(cfg, &mockTranscriber{result: "test text"}, &postprocess.NoopPostProcessor{}, nil, nil, nil, log.New(io.Discard, "", 0), false)
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

func TestBlankAudioTranscriptionSkipsPaste(t *testing.T) {
	m := newTestModel()
	m.State = StateTranscribing
	updated, cmd := m.Update(TranscriptionResultMsg{Text: "[BLANK_AUDIO]"})
	model := updated.(Model)
	if model.State != StateIdle {
		t.Errorf("expected StateIdle, got %d", model.State)
	}
	if cmd != nil {
		t.Error("expected no command for blank audio")
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

func TestCustomThemeRegistrationAndCycle(t *testing.T) {
	// Reset theme state to avoid pollution from other tests.
	// Save and restore themeOrder/themes after the test.
	origOrder := make([]string, len(themeOrder))
	copy(origOrder, themeOrder)
	origThemes := make(map[string]Theme)
	for k, v := range themes {
		origThemes[k] = v
	}
	defer func() {
		themeOrder = origOrder
		themes = origThemes
	}()

	cfg := config.Default()
	cfg.Theme = "testcustom"
	cfg.CustomThemes = []config.CustomTheme{
		{
			Name:       "testcustom",
			Primary:    "#008585",
			Secondary:  "#74A892",
			Accent:     "#C7522A",
			Error:      "#C7522A",
			Success:    "#74A892",
			Warning:    "#D97706",
			Background: "#1A1611",
			Text:       "#FEF9E0",
			Dimmed:     "#535A63",
			Separator:  "#625647",
		},
	}

	m := NewModel(cfg, &mockTranscriber{result: "test"}, &postprocess.NoopPostProcessor{}, nil, nil, nil, log.New(io.Discard, "", 0), false)

	// Theme should be loaded and active.
	if m.themeName != "testcustom" {
		t.Errorf("expected themeName testcustom, got %s", m.themeName)
	}

	// The custom theme should be loadable.
	loaded := LoadTheme("testcustom")
	if loaded.Name != "testcustom" {
		t.Errorf("expected loaded theme name testcustom, got %s", loaded.Name)
	}
	if string(loaded.Primary) != "#008585" {
		t.Errorf("expected primary #008585, got %s", string(loaded.Primary))
	}

	// The custom theme should appear in the cycle.
	found := false
	for _, name := range ThemeNames() {
		if name == "testcustom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected testcustom to appear in ThemeNames()")
	}

	// Cycling from monochrome should reach the custom theme.
	next := NextTheme("monochrome")
	if strings.ToLower(next.Name) != "testcustom" {
		t.Errorf("expected NextTheme(monochrome) to be testcustom, got %s", next.Name)
	}
}

func TestCustomThemeSkipsBuiltinCollision(t *testing.T) {
	origOrder := make([]string, len(themeOrder))
	copy(origOrder, themeOrder)
	origThemes := make(map[string]Theme)
	for k, v := range themes {
		origThemes[k] = v
	}
	defer func() {
		themeOrder = origOrder
		themes = origThemes
	}()

	custom := []config.CustomTheme{
		{Name: "synthwave", Primary: "#FF0000"},
		{Name: "", Primary: "#FF0000"},
	}
	RegisterCustomThemes(custom)

	// Built-in synthwave should not be overridden.
	if string(themes["synthwave"].Primary) != "#FF6AC1" {
		t.Error("expected built-in synthwave to be unchanged")
	}
	// Theme order should not have grown.
	if len(themeOrder) != 4 {
		t.Errorf("expected 4 themes in order, got %d", len(themeOrder))
	}
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

func TestTranscriptionResultWithPostProcessing(t *testing.T) {
	m := newTestModel()
	m.State = StateTranscribing
	m.Config.PostProcessing.Enabled = true
	m.toneName = "polite"
	m.PostProcessor = &mockPostProcessor{result: "please help me"}
	updated, cmd := m.Update(TranscriptionResultMsg{Text: "help me"})
	model := updated.(Model)
	if model.State != StatePostProcessing {
		t.Errorf("expected StatePostProcessing, got %d", model.State)
	}
	if cmd == nil {
		t.Error("expected post-process command")
	}
}

func TestTranscriptionResultWithoutPostProcessing(t *testing.T) {
	m := newTestModel()
	m.State = StateTranscribing
	m.Config.PostProcessing.Enabled = false
	m.toneName = "off"
	updated, cmd := m.Update(TranscriptionResultMsg{Text: "hello world"})
	model := updated.(Model)
	if model.State != StatePasting {
		t.Errorf("expected StatePasting, got %d", model.State)
	}
	if cmd == nil {
		t.Error("expected paste command")
	}
}

func TestTranscriptionResultWithToneOff(t *testing.T) {
	m := newTestModel()
	m.State = StateTranscribing
	m.Config.PostProcessing.Enabled = true
	m.toneName = "off"
	updated, _ := m.Update(TranscriptionResultMsg{Text: "hello"})
	model := updated.(Model)
	if model.State != StatePasting {
		t.Errorf("expected StatePasting when tone is off, got %d", model.State)
	}
}

func TestPostProcessResultTransition(t *testing.T) {
	m := newTestModel()
	m.State = StatePostProcessing
	updated, cmd := m.Update(PostProcessResultMsg{Text: "rewritten text", OriginalText: "original"})
	model := updated.(Model)
	if model.State != StatePasting {
		t.Errorf("expected StatePasting, got %d", model.State)
	}
	if cmd == nil {
		t.Error("expected paste command")
	}
}

func TestPostProcessErrorGracefulDegradation(t *testing.T) {
	m := newTestModel()
	m.State = StatePostProcessing
	updated, cmd := m.Update(PostProcessErrorMsg{Err: fmt.Errorf("timeout"), OriginalText: "original text"})
	model := updated.(Model)
	if model.State != StatePasting {
		t.Errorf("expected StatePasting on PP error (graceful degradation), got %d", model.State)
	}
	if cmd == nil {
		t.Error("expected paste command with original text")
	}
}

func TestToneCycleKeyP(t *testing.T) {
	defer postprocess.ResetTones()

	m := newTestModel()
	m.toneName = "off"
	m.Config.PostProcessing.Tone = "off"
	updated, cmd := m.Update(testKeyMsg("p"))
	model := updated.(Model)
	if model.toneName != "formal" {
		t.Errorf("expected tone formal after cycling from off, got %s", model.toneName)
	}
	if !model.Config.PostProcessing.Enabled {
		t.Error("expected post-processing enabled after cycling to formal")
	}
	if cmd == nil {
		t.Error("expected save config command")
	}
}

func TestToneCycleKeyPToOff(t *testing.T) {
	defer postprocess.ResetTones()
	m := newTestModel()
	// Find the last tone before "off" in the cycle
	names := postprocess.ToneNames()
	lastTone := names[len(names)-1]
	m.toneName = lastTone
	m.Config.PostProcessing.Tone = lastTone
	m.Config.PostProcessing.Enabled = true
	updated, _ := m.Update(testKeyMsg("p"))
	model := updated.(Model)
	if model.toneName != "off" {
		t.Errorf("expected tone off after cycling from last tone, got %s", model.toneName)
	}
	if model.Config.PostProcessing.Enabled {
		t.Error("expected post-processing disabled after cycling to off")
	}
}

func TestModelCycleKeyM(t *testing.T) {
	m := newTestModel()
	m.toneName = "polite"
	m.Config.PostProcessing.Enabled = true
	m.ppModels = []string{"llama3.2", "mistral", "codellama"}
	m.ppModelName = "llama3.2"
	m.Config.PostProcessing.Model = "llama3.2"
	updated, cmd := m.Update(testKeyMsg("m"))
	model := updated.(Model)
	if model.ppModelName != "mistral" {
		t.Errorf("expected model mistral after cycling, got %s", model.ppModelName)
	}
	if model.Config.PostProcessing.Model != "mistral" {
		t.Errorf("expected config model mistral, got %s", model.Config.PostProcessing.Model)
	}
	if cmd == nil {
		t.Error("expected save config + list models command")
	}
}

func TestModelCycleKeyMIgnoredWhenOff(t *testing.T) {
	m := newTestModel()
	m.toneName = "off"
	m.ppModels = []string{"llama3.2", "mistral"}
	m.ppModelName = "llama3.2"
	updated, cmd := m.Update(testKeyMsg("m"))
	model := updated.(Model)
	if model.ppModelName != "llama3.2" {
		t.Errorf("expected model unchanged when tone off, got %s", model.ppModelName)
	}
	if cmd != nil {
		t.Error("expected no command when tone is off")
	}
}

func TestModelCycleKeyMIgnoredWhenNoModels(t *testing.T) {
	m := newTestModel()
	m.toneName = "polite"
	m.ppModels = nil
	m.ppModelName = "llama3.2"
	updated, cmd := m.Update(testKeyMsg("m"))
	model := updated.(Model)
	if model.ppModelName != "llama3.2" {
		t.Errorf("expected model unchanged when no models, got %s", model.ppModelName)
	}
	if cmd != nil {
		t.Error("expected no command when no models available")
	}
}

func TestViewShowsRewritingBadge(t *testing.T) {
	m := newTestModel()
	m.State = StatePostProcessing
	view := m.View()
	if !contains(view, "Rewriting") {
		t.Error("expected view to contain 'Rewriting' badge")
	}
}

func TestViewShowsToneInFooter(t *testing.T) {
	m := newTestModel()
	m.toneName = "polite"
	view := m.View()
	if !contains(view, "p: tone (polite)") {
		t.Error("expected footer to contain 'p: tone (polite)'")
	}
}

func TestViewShowsModelInFooterWhenActive(t *testing.T) {
	m := newTestModel()
	m.toneName = "formal"
	m.ppModelName = "llama3.2"
	m.Config.PostProcessing.Enabled = true
	view := m.View()
	if !contains(view, "m: model") || !contains(view, "llama3.2") {
		t.Error("expected footer to contain model info when tone is active")
	}
}

func TestViewHidesModelInFooterWhenOff(t *testing.T) {
	m := newTestModel()
	m.toneName = "off"
	m.ppModelName = "llama3.2"
	view := m.View()
	if contains(view, "m: model") {
		t.Error("expected footer to NOT contain 'm: model' when tone is off")
	}
}

func TestPPModelsListMsgUpdatesModels(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(PPModelsListMsg{Models: []string{"llama3.2", "mistral"}})
	model := updated.(Model)
	if len(model.ppModels) != 2 {
		t.Fatalf("expected 2 PP models, got %d", len(model.ppModels))
	}
	if model.ppModels[0] != "llama3.2" {
		t.Errorf("expected first model llama3.2, got %s", model.ppModels[0])
	}
}

func TestPPModelsListMsgErrorPreservesExisting(t *testing.T) {
	m := newTestModel()
	m.ppModels = []string{"existing"}
	updated, _ := m.Update(PPModelsListMsg{Err: fmt.Errorf("connection refused")})
	model := updated.(Model)
	if len(model.ppModels) != 1 || model.ppModels[0] != "existing" {
		t.Error("expected existing models preserved on error")
	}
}

func TestPPModelsListAutoSelectsWhenConfiguredNotFound(t *testing.T) {
	m := newTestModel()
	m.ppModelName = "nonexistent-model"
	m.Config.PostProcessing.Model = "nonexistent-model"
	m.toneName = "polite"
	m.Config.PostProcessing.Enabled = true
	updated, cmd := m.Update(PPModelsListMsg{Models: []string{"llama3.2:3b", "mistral"}})
	model := updated.(Model)
	if model.ppModelName != "llama3.2:3b" {
		t.Errorf("expected auto-selected model llama3.2:3b, got %s", model.ppModelName)
	}
	if model.Config.PostProcessing.Model != "llama3.2:3b" {
		t.Errorf("expected config model llama3.2:3b, got %s", model.Config.PostProcessing.Model)
	}
	if cmd == nil {
		t.Error("expected save config command after auto-selection")
	}
}

func TestPPModelsListKeepsConfiguredWhenFound(t *testing.T) {
	m := newTestModel()
	m.ppModelName = "mistral"
	m.Config.PostProcessing.Model = "mistral"
	updated, cmd := m.Update(PPModelsListMsg{Models: []string{"llama3.2:3b", "mistral"}})
	model := updated.(Model)
	if model.ppModelName != "mistral" {
		t.Errorf("expected model to stay mistral, got %s", model.ppModelName)
	}
	if cmd != nil {
		t.Error("expected no save command when model is already correct")
	}
}

// testKeyMsg creates a tea.KeyMsg for single-rune keys like "p", "m", "t".
func testKeyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func TestTranscriptKeyCEntersMode(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel()
	rec := &mockContinuousRecorder{}
	m.Recorder = rec
	m.TranscriptOutput = filepath.Join(dir, "out.txt")

	updated, cmd := m.Update(testKeyMsg("c"))
	model := updated.(Model)
	if model.State != StateTranscript {
		t.Errorf("expected StateTranscript, got %d", model.State)
	}
	if !rec.continuous {
		t.Error("expected StartContinuous to be called")
	}
	if cmd == nil {
		t.Error("expected tick commands on enter")
	}
	if model.TranscriptPath != m.TranscriptOutput {
		t.Errorf("expected path %q, got %q", m.TranscriptOutput, model.TranscriptPath)
	}
}

func TestTranscriptKeyCIgnoredWhileRecording(t *testing.T) {
	m := newTestModel()
	m.State = StateRecording
	rec := &mockContinuousRecorder{}
	m.Recorder = rec
	updated, cmd := m.Update(testKeyMsg("c"))
	model := updated.(Model)
	if model.State != StateRecording {
		t.Errorf("expected StateRecording, got %d", model.State)
	}
	if rec.continuous {
		t.Error("expected transcript mode not to start during hold-to-talk")
	}
	if cmd != nil {
		t.Error("expected no command")
	}
}

func TestTranscriptResultDoesNotPaste(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	m := newTestModel()
	rec := &mockContinuousRecorder{}
	m.Recorder = rec
	m.TranscriptOutput = path
	updated, _ := m.Update(testKeyMsg("c"))
	m = updated.(Model)
	m.transcriptBusy = true

	updated, cmd := m.Update(TranscriptionResultMsg{Text: "hello from mic"})
	m = updated.(Model)
	if m.State != StateTranscript {
		t.Errorf("expected StateTranscript, got %d", m.State)
	}
	if m.LastTranscript != "hello from mic" {
		t.Errorf("expected last transcript, got %q", m.LastTranscript)
	}
	if cmd != nil {
		t.Errorf("expected no paste command, got %T", cmd)
	}

	data, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello from mic\n" {
		t.Errorf("file contents %q", data)
	}
}

func TestTranscriptBlankAudioDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	m := newTestModel()
	rec := &mockContinuousRecorder{}
	m.Recorder = rec
	m.TranscriptOutput = path
	updated, _ := m.Update(testKeyMsg("c"))
	m = updated.(Model)
	m.transcriptBusy = true

	updated, cmd := m.Update(TranscriptionResultMsg{Text: "[BLANK_AUDIO]"})
	m = updated.(Model)
	if m.State != StateTranscript {
		t.Errorf("expected StateTranscript, got %d", m.State)
	}
	if m.LastTranscript != "" {
		t.Errorf("expected empty last transcript, got %q", m.LastTranscript)
	}
	if cmd != nil {
		t.Error("expected no command")
	}

	data, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file, got %q", data)
	}
}

func TestTranscriptKeyCExitsMode(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel()
	rec := &mockContinuousRecorder{stopErr: fmt.Errorf("no audio captured")}
	m.Recorder = rec
	m.TranscriptOutput = filepath.Join(dir, "out.txt")
	updated, _ := m.Update(testKeyMsg("c"))
	m = updated.(Model)

	updated, cmd := m.Update(testKeyMsg("c"))
	m = updated.(Model)
	if m.State != StateIdle {
		t.Errorf("expected StateIdle, got %d", m.State)
	}
	if !rec.stopped {
		t.Error("expected Stop to be called")
	}
	if m.transcriptWriter != nil {
		t.Error("expected writer closed")
	}
	if cmd != nil {
		t.Error("expected no quit command when leaving without leftover audio")
	}
}

func TestTranscriptGateSetAndCleared(t *testing.T) {
	dir := t.TempDir()
	var gate atomic.Bool
	m := newTestModel()
	rec := &mockContinuousRecorder{stopErr: fmt.Errorf("no audio captured")}
	m.Recorder = rec
	m.TranscriptGate = &gate
	m.TranscriptOutput = filepath.Join(dir, "out.txt")

	updated, _ := m.Update(testKeyMsg("c"))
	m = updated.(Model)
	if !gate.Load() {
		t.Error("expected gate set while in transcript mode")
	}

	updated, _ = m.Update(testKeyMsg("c"))
	m = updated.(Model)
	if gate.Load() {
		t.Error("expected gate cleared after leaving transcript mode")
	}
}

func TestTranscriptChunkTickQueuesWhenBusy(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel()
	rec := &mockContinuousRecorder{takeWAV: []byte("wav")}
	m.Recorder = rec
	m.TranscriptOutput = filepath.Join(dir, "out.txt")
	updated, _ := m.Update(testKeyMsg("c"))
	m = updated.(Model)
	m.transcriptBusy = true

	updated, cmd := m.Update(transcriptChunkTickMsg{})
	m = updated.(Model)
	if len(m.transcriptQueue) != 1 {
		t.Fatalf("expected 1 queued chunk, got %d", len(m.transcriptQueue))
	}
	if cmd == nil {
		t.Error("expected chunk tick to reschedule")
	}
}

func TestAudioLevelTickUpdatesLevelInTranscript(t *testing.T) {
	m := newTestModel()
	m.State = StateTranscript
	m.Recorder = &mockLevelSampler{level: 0.3}
	updated, cmd := m.Update(audioLevelTickMsg{})
	model := updated.(Model)
	if model.AudioLevel != 0.3 {
		t.Errorf("expected AudioLevel 0.3, got %f", model.AudioLevel)
	}
	if cmd == nil {
		t.Error("expected another tick command while in transcript mode")
	}
}

func TestViewShowsTranscriptBadgeAndFooter(t *testing.T) {
	m := newTestModel()
	m.State = StateTranscript
	m.TranscriptPath = "palaver-transcript-test.txt"
	view := m.View()
	if !contains(view, "Transcript") {
		t.Error("expected view to contain Transcript badge")
	}
	if !contains(view, "c: transcript") && !contains(view, "c: stop") {
		t.Error("expected view to contain transcript toggle hint")
	}
	if !contains(view, "Writing:") {
		t.Error("expected view to contain Writing path")
	}
}

func TestTranscriptKeyCWithoutRecorderErrors(t *testing.T) {
	m := newTestModel()
	updated, cmd := m.Update(testKeyMsg("c"))
	model := updated.(Model)
	if model.State != StateError {
		t.Errorf("expected StateError, got %d", model.State)
	}
	if cmd == nil {
		t.Error("expected error timeout command")
	}
}
