package tui

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Danondso/palaver/internal/chime"
	"github.com/Danondso/palaver/internal/clipboard"
	"github.com/Danondso/palaver/internal/config"
	"github.com/Danondso/palaver/internal/postprocess"
	"github.com/Danondso/palaver/internal/server"
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
	StatePostProcessing
	StatePasting
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

type PostProcessResultMsg struct {
	Text         string
	OriginalText string
	NeedsSpace   bool
}

type PostProcessErrorMsg struct {
	Err          error
	OriginalText string
}

type PPModelsListMsg struct {
	Models []string
	Err    error
}

type PasteDoneMsg struct{ Err error }

type errorTimeoutMsg struct{}

type configSavedMsg struct{ err error }

type audioLevelTickMsg struct{}

// StatusCheckMsg carries the result of a mic + backend availability check.
type StatusCheckMsg struct {
	MicDetected   bool
	BackendOnline bool
	MicDeviceName string
	ModelName     string
}

type statusCheckTickMsg struct{}

// ServerStateMsg carries a server lifecycle state update.
type ServerStateMsg struct {
	State  string // "starting", "running", "stopped", "error"
	Detail string
}

type serverStartDoneMsg struct{ err error }
type serverStartingMsg struct{}

// DebugEntry is a structured debug log entry.
type DebugEntry struct {
	Time     string // e.g. "11:27:53"
	Category string // e.g. "hotkey", "paste", "transcribe"
	Message  string // the log message
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
	ModelName      string
	statusChecked  bool
	themeName      string
	PostProcessor  postprocess.PostProcessor
	toneName       string
	ppModelName    string
	ppModels       []string
	Server         *server.Server     // nil if not using managed server
	serverState    string             // "", "starting", "running", "stopped", "error"
	ServerCtx      context.Context    // cancellable context for server operations
	ServerCancel   context.CancelFunc // cancel function for ServerCtx
}

// NewModel creates a new TUI model.
func NewModel(cfg *config.Config, t transcriber.Transcriber, pp postprocess.PostProcessor, c *chime.Player, rec LevelSampler, mc MicChecker, logger *log.Logger, debug bool) Model {
	RegisterCustomThemes(cfg.CustomThemes)
	themeName := cfg.Theme
	applyTheme(LoadTheme(themeName))
	return Model{
		State:         StateIdle,
		Config:        cfg,
		Transcriber:   t,
		PostProcessor: pp,
		Chime:         c,
		Recorder:      rec,
		MicChecker:    mc,
		HotkeyName:    cfg.Hotkey.Key,
		Logger:        logger,
		DebugMode:     debug,
		themeName:     themeName,
		toneName:      cfg.PostProcessing.Tone,
		ppModelName:   cfg.PostProcessing.Model,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.statusCheckCmd()}
	if m.Server != nil {
		cmds = append(cmds, func() tea.Msg { return serverStartingMsg{} })
		cmds = append(cmds, m.ServerStartCmd())
	}
	if m.Config.PostProcessing.Enabled && strings.ToLower(m.toneName) != "off" {
		cmds = append(cmds, m.ppListModelsCmd())
	}
	return tea.Batch(cmds...)
}

// Update handles messages and transitions state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// During pasting, simulated keystrokes from xdotool/ydotool may
		// feed back into the TUI. Only allow ctrl+c to avoid unintended
		// theme toggles, quits, etc.
		if m.State == StatePasting {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "t":
			next := NextTheme(m.themeName)
			applyTheme(next)
			m.themeName = strings.ToLower(next.Name)
			m.Config.Theme = m.themeName
			return m, m.saveConfigCmd()
		case "p":
			next := postprocess.NextTone(m.toneName)
			m.toneName = next
			m.Config.PostProcessing.Tone = next
			if next == "off" {
				m.Config.PostProcessing.Enabled = false
				m.PostProcessor = &postprocess.NoopPostProcessor{}
				return m, m.saveConfigCmd()
			}
			m.Config.PostProcessing.Enabled = true
			m.rebuildPostProcessor()
			return m, tea.Batch(m.saveConfigCmd(), m.ppListModelsCmd())
		case "m":
			if m.Config.PostProcessing.Enabled && strings.ToLower(m.toneName) != "off" && len(m.ppModels) > 0 {
				currentIdx := -1
				for i, name := range m.ppModels {
					if name == m.ppModelName {
						currentIdx = i
						break
					}
				}
				nextIdx := (currentIdx + 1) % len(m.ppModels)
				m.ppModelName = m.ppModels[nextIdx]
				m.Config.PostProcessing.Model = m.ppModelName
				m.rebuildPostProcessor()
				return m, tea.Batch(m.saveConfigCmd(), m.ppListModelsCmd())
			}
		case "r":
			if m.Server != nil {
				m.serverState = "starting"
				return m, m.serverRestartCmd()
			}
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
		if msg.ModelName != "" {
			m.ModelName = msg.ModelName
		}
		m.statusChecked = true
		return m, scheduleStatusRecheck()

	case statusCheckTickMsg:
		return m, m.statusCheckCmd()

	case TranscriptionResultMsg:
		text := msg.Text
		m.Logger.Printf("transcription result: %q", text)
		if text == "" || text == "[BLANK_AUDIO]" {
			m.State = StateIdle
			m.Logger.Printf("empty transcription, skipping paste")
			return m, nil
		}
		needsSpace := m.LastTranscript != ""
		m.LastTranscript = msg.Text
		// Post-processing gate
		if m.Config.PostProcessing.Enabled && strings.ToLower(m.toneName) != "off" {
			m.State = StatePostProcessing
			return m, m.postProcessCmd(text, needsSpace)
		}
		// Add a leading space between consecutive transcriptions.
		if needsSpace {
			text = " " + text
		}
		m.State = StatePasting
		return m, m.pasteCmd(text)

	case PostProcessResultMsg:
		m.Logger.Printf("post-processing result: %q", msg.Text)
		text := msg.Text
		// Add a leading space between consecutive transcriptions (after rewriting).
		if msg.NeedsSpace {
			text = " " + text
		}
		m.State = StatePasting
		return m, m.pasteCmd(text)

	case PostProcessErrorMsg:
		m.Logger.Printf("post-processing error (falling back to original): %v", msg.Err)
		m.State = StatePasting
		return m, m.pasteCmd(msg.OriginalText)

	case PPModelsListMsg:
		if msg.Err != nil {
			m.Logger.Printf("failed to list post-processing models: %v", msg.Err)
		} else {
			m.ppModels = msg.Models
			// If the configured model isn't in the list, auto-select the first available.
			if len(msg.Models) > 0 {
				found := false
				for _, name := range msg.Models {
					if name == m.ppModelName {
						found = true
						break
					}
				}
				if !found {
					m.Logger.Printf("configured post-processing model %q not found, using %q", m.ppModelName, msg.Models[0])
					m.ppModelName = msg.Models[0]
					m.Config.PostProcessing.Model = m.ppModelName
					if strings.ToLower(m.toneName) != "off" {
						m.rebuildPostProcessor()
					}
					return m, m.saveConfigCmd()
				}
			}
		}

	case PasteDoneMsg:
		if msg.Err != nil {
			m.State = StateError
			m.LastError = msg.Err.Error()
			return m, scheduleErrorTimeout()
		}
		m.State = StateIdle
		return m, nil

	case TranscriptionErrorMsg:
		m.State = StateError
		m.LastError = msg.Err.Error()
		return m, scheduleErrorTimeout()

	case errorTimeoutMsg:
		m.State = StateIdle
		m.LastError = ""

	case serverStartingMsg:
		m.serverState = "starting"

	case ServerStateMsg:
		m.serverState = msg.State

	case serverStartDoneMsg:
		if msg.err != nil {
			m.serverState = "error"
			m.Logger.Printf("server start failed: %v", msg.err)
		} else {
			m.serverState = "running"
		}
		return m, m.statusCheckCmd()

	case configSavedMsg:
		if msg.err != nil && m.Logger != nil {
			m.Logger.Printf("failed to save config: %v", msg.err)
		}

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
			return PasteDoneMsg{Err: fmt.Errorf("paste: %w", err)}
		}
		logger.Printf("paste: success")
		return PasteDoneMsg{}
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
		modelName := ""
		if hc, ok := t.(transcriber.HealthChecker); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			backendOk = hc.Ping(ctx) == nil
		}
		if ml, ok := t.(transcriber.ModelLister); ok && backendOk {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if models, err := ml.ListModels(ctx); err == nil && len(models) > 0 {
				modelName = models[0]
			}
		}
		// Fall back to configured model name when ListModels is unavailable
		if modelName == "" && backendOk {
			if cm, ok := t.(transcriber.ConfiguredModeler); ok {
				modelName = cm.ConfiguredModel()
			}
		}
		return StatusCheckMsg{MicDetected: micOk, BackendOnline: backendOk, MicDeviceName: micName, ModelName: modelName}
	}
}

func scheduleStatusRecheck() tea.Cmd {
	return tea.Tick(statusRecheckInterval, func(time.Time) tea.Msg {
		return statusCheckTickMsg{}
	})
}

func (m Model) saveConfigCmd() tea.Cmd {
	cfg := m.Config
	path := config.DefaultPath()
	return func() tea.Msg {
		return configSavedMsg{err: config.Save(path, cfg)}
	}
}

func (m Model) serverRestartCmd() tea.Cmd {
	srv := m.Server
	ctx := m.ServerCtx
	return func() tea.Msg {
		err := srv.Restart(ctx)
		return serverStartDoneMsg{err: err}
	}
}

// ServerStartCmd returns a tea.Cmd that starts the managed server.
func (m Model) ServerStartCmd() tea.Cmd {
	srv := m.Server
	ctx := m.ServerCtx
	return func() tea.Msg {
		err := srv.Start(ctx)
		return serverStartDoneMsg{err: err}
	}
}

// rebuildPostProcessor creates a new LLMPostProcessor from the current config.
func (m *Model) rebuildPostProcessor() {
	tone := postprocess.ResolveTone(m.toneName)
	m.PostProcessor = postprocess.NewLLM(
		m.Config.PostProcessing.BaseURL,
		m.ppModelName,
		tone.Prompt,
		m.Config.PostProcessing.TimeoutSec,
		m.Logger,
	)
}

func (m Model) postProcessCmd(text string, needsSpace bool) tea.Cmd {
	pp := m.PostProcessor
	return func() tea.Msg {
		ctx := context.Background()
		result, err := pp.Rewrite(ctx, text)
		if err != nil {
			return PostProcessErrorMsg{Err: err, OriginalText: text}
		}
		return PostProcessResultMsg{Text: result, OriginalText: text, NeedsSpace: needsSpace}
	}
}

func (m Model) ppListModelsCmd() tea.Cmd {
	pp := m.PostProcessor
	return func() tea.Msg {
		if ml, ok := pp.(postprocess.ModelLister); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			models, err := ml.ListModels(ctx)
			return PPModelsListMsg{Models: models, Err: err}
		}
		return PPModelsListMsg{}
	}
}
