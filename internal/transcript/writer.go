package transcript

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Writer appends transcribed lines to a text file.
type Writer struct {
	path string
	f    *os.File
}

// DefaultFilename returns palaver-transcript-YYYYMMDD-HHMMSS.txt.
func DefaultFilename(t time.Time) string {
	return fmt.Sprintf("palaver-transcript-%s.txt", t.Format("20060102-150405"))
}

// Open creates or appends to path.
func Open(path string) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // path is user-supplied transcript output
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	return &Writer{path: path, f: f}, nil
}

// Path returns the file path being written.
func (w *Writer) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}

// Append writes a non-blank transcript line and syncs to disk.
func (w *Writer) Append(text string) error {
	if w == nil || w.f == nil {
		return fmt.Errorf("transcript writer is closed")
	}
	text = strings.TrimSpace(text)
	if text == "" || text == "[BLANK_AUDIO]" {
		return nil
	}
	if _, err := fmt.Fprintln(w.f, text); err != nil {
		return fmt.Errorf("write transcript: %w", err)
	}
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("sync transcript: %w", err)
	}
	return nil
}

// Close closes the underlying file.
func (w *Writer) Close() error {
	if w == nil || w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
