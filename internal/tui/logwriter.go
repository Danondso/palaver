package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// LogWriter is an io.Writer that sends each written line as a DebugLogMsg
// to a Bubble Tea program. Use it as the output for a log.Logger.
type LogWriter struct {
	program *tea.Program
}

// NewLogWriter creates a LogWriter that sends debug lines to the given program.
func NewLogWriter(p *tea.Program) *LogWriter {
	return &LogWriter{program: p}
}

// Write implements io.Writer. Each call parses the log line into structured
// fields and sends a DebugLogMsg. The send is done in a goroutine to avoid
// deadlocking when called from inside a Bubble Tea command function.
func (w *LogWriter) Write(b []byte) (int, error) {
	line := strings.TrimRight(string(b), "\n")
	entry := parseLine(line)
	go w.program.Send(DebugLogMsg{Entry: entry})
	return len(b), nil
}

// parseLine extracts time, category, and message from a log line.
// Expected format: "[DEBUG] HH:MM:SS.micros message text"
// Category is inferred from the first word of the message (e.g. "hotkey",
// "transcribe", "paste", "recording", "portaudio", "keyboard").
func parseLine(line string) DebugEntry {
	entry := DebugEntry{
		Time:     "",
		Category: "debug",
		Message:  line,
	}

	// Strip "[DEBUG] " prefix
	msg := strings.TrimPrefix(line, "[DEBUG] ")

	// Extract timestamp (HH:MM:SS.micros or HH:MM:SS)
	if len(msg) >= 8 && msg[2] == ':' && msg[5] == ':' {
		// Find the end of the timestamp (space after time)
		spaceIdx := strings.IndexByte(msg, ' ')
		if spaceIdx > 0 {
			entry.Time = msg[:spaceIdx]
			msg = msg[spaceIdx+1:]
		}
	}

	// Infer category from message prefix
	entry.Category, entry.Message = inferCategory(msg)

	return entry
}

// inferCategory determines the log category from the message content.
func inferCategory(msg string) (category, message string) {
	lower := strings.ToLower(msg)

	switch {
	case strings.HasPrefix(lower, "hotkey"):
		return "hotkey", msg
	case strings.HasPrefix(lower, "transcrib"), strings.HasPrefix(lower, "transcription"):
		return "transcribe", msg
	case strings.HasPrefix(lower, "paste"):
		return "paste", msg
	case strings.HasPrefix(lower, "recording"), strings.HasPrefix(lower, "recorder"):
		return "recorder", msg
	case strings.HasPrefix(lower, "portaudio"):
		return "audio", msg
	case strings.HasPrefix(lower, "keyboard"):
		return "device", msg
	case strings.HasPrefix(lower, "empty"):
		return "transcribe", msg
	default:
		return "debug", msg
	}
}
