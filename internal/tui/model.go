package tui

import (
	"context"
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Danondso/palaver/internal/chime"
	"github.com/Danondso/palaver/internal/clipboard"
	"github.com/Danondso/palaver/internal/config"
	"github.com/Danondso/palaver/internal/transcriber"
)

// LevelSampler can report the current audio amplitude level.
type LevelSampler interface {
	AudioLevel() float64
}

// MicChecker can report whether a microphone input device is available.
type MicChecker interface {
	MicAvailable() bool
	MicName() string
}

// State represents the application state.
type State int

const (
	StateIdle State = iota
	StateRecording
	StateTranscribing
	StateError
)

// Messages sent through the Bubble Tea update loop.

type RecordingStartedMsg struct{}

type RecordingStoppedMsg struct {
	WavData []byte
}

type TranscriptionResultMsg struct {
	Text string
}

type TranscriptionErrorMsg struct {
	Err error
}

type errorTimeoutMsg struct{}

type audioLevelTickMsg struct{}

// StatusCheckMsg carries the result of a mic + backend availability check.
type StatusCheckMsg struct {
	MicDetected   bool
	BackendOnline bool
	MicDeviceName string
}

type statusCheckTickMsg struct{}

// DebugEntry is a structured debug log entry.
type DebugEntry struct {
	Time    string // e.g. "11:27:53"
	Category string // e.g. "hotkey", "paste", "transcribe"
	Message string // the log message
}

// DebugLogMsg carries a structured debug log entry into the TUI.
type DebugLogMsg struct {
	Entry DebugEntry
}

const maxDebugLines = 50

// Model is the Bubble Tea model for the Palaver TUI.
type Model struct {
	State          State
	LastTranscript string
	LastError      string
	Config         *config.Config
	Transcriber    transcriber.Transcriber
	Chime          *chime.Player
	HotkeyName     string
	Logger         *log.Logger
	DebugMode      bool
	DebugEntries   []DebugEntry
	AudioLevel     float64
	Recorder       LevelSampler
	MicChecker     MicChecker
	MicDetected    bool
	MicDeviceName  string
	BackendOnline  bool
	statusChecked  bool
}

// NewModel creates a new TUI model.
func NewModel(cfg *config.Config, t transcriber.Transcriber, c *chime.Player, rec LevelSampler, mc MicChecker, logger *log.Logger, debug bool) Model {
	return Model{
		State:       StateIdle,
		Config:      cfg,
		Transcriber: t,
		Chime:       c,
		Recorder:    rec,
		MicChecker:  mc,
		HotkeyName:  cfg.Hotkey.Key,
		Logger:      logger,
		DebugMode:   debug,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return m.statusCheckCmd()
}

// Update handles messages and transitions state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case RecordingStartedMsg:
		m.State = StateRecording
		m.LastError = ""
		if m.Chime != nil {
			m.Chime.PlayStart()
		}
		return m, audioLevelTickCmd()

	case audioLevelTickMsg:
		if m.State == StateRecording && m.Recorder != nil {
			m.AudioLevel = m.Recorder.AudioLevel()
			return m, audioLevelTickCmd()
		}
		m.AudioLevel = 0
		return m, nil

	case RecordingStoppedMsg:
		m.State = StateTranscribing
		m.AudioLevel = 0
		if m.Chime != nil {
			m.Chime.PlayStop()
		}
		return m, m.transcribeCmd(msg.WavData)

	case StatusCheckMsg:
		m.MicDetected = msg.MicDetected
		m.MicDeviceName = msg.MicDeviceName
		m.BackendOnline = msg.BackendOnline
		m.statusChecked = true
		return m, scheduleStatusRecheck()

	case statusCheckTickMsg:
		return m, m.statusCheckCmd()

	case TranscriptionResultMsg:
		m.State = StateIdle
		m.LastTranscript = msg.Text
		m.Logger.Printf("transcription result: %q", msg.Text)
		if msg.Text == "" {
			m.Logger.Printf("empty transcription, skipping paste")
			return m, nil
		}
		return m, m.pasteCmd(msg.Text)

	case TranscriptionErrorMsg:
		m.State = StateError
		m.LastError = msg.Err.Error()
		return m, scheduleErrorTimeout()

	case errorTimeoutMsg:
		m.State = StateIdle
		m.LastError = ""

	case DebugLogMsg:
		m.DebugEntries = append(m.DebugEntries, msg.Entry)
		if len(m.DebugEntries) > maxDebugLines {
			m.DebugEntries = m.DebugEntries[len(m.DebugEntries)-maxDebugLines:]
		}
	}

	return m, nil
}

func (m Model) transcribeCmd(wavData []byte) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		text, err := m.Transcriber.Transcribe(ctx, wavData)
		if err != nil {
			return TranscriptionErrorMsg{Err: err}
		}
		return TranscriptionResultMsg{Text: text}
	}
}

func (m Model) pasteCmd(text string) tea.Cmd {
	delayMs := m.Config.Paste.DelayMs
	mode := m.Config.Paste.Mode
	logger := m.Logger
	return func() tea.Msg {
		logger.Printf("paste: mode=%s delay=%dms", mode, delayMs)
		if err := clipboard.PasteText(text, delayMs, mode); err != nil {
			logger.Printf("paste error: %v", err)
			return TranscriptionErrorMsg{Err: fmt.Errorf("paste: %w", err)}
		}
		logger.Printf("paste: success")
		return nil
	}
}

func scheduleErrorTimeout() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return errorTimeoutMsg{}
	})
}

const audioLevelTickInterval = 100 * time.Millisecond

func audioLevelTickCmd() tea.Cmd {
	return tea.Tick(audioLevelTickInterval, func(time.Time) tea.Msg {
		return audioLevelTickMsg{}
	})
}

const statusRecheckInterval = 30 * time.Second

func (m Model) statusCheckCmd() tea.Cmd {
	t := m.Transcriber
	mc := m.MicChecker
	return func() tea.Msg {
		micOk := false
		micName := ""
		if mc != nil {
			micOk = mc.MicAvailable()
			micName = mc.MicName()
		}
		backendOk := false
		if hc, ok := t.(transcriber.HealthChecker); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			backendOk = hc.Ping(ctx) == nil
		}
		return StatusCheckMsg{MicDetected: micOk, BackendOnline: backendOk, MicDeviceName: micName}
	}
}

func scheduleStatusRecheck() tea.Cmd {
	return tea.Tick(statusRecheckInterval, func(time.Time) tea.Msg {
		return statusCheckTickMsg{}
	})
}
