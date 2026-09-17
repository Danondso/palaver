package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultFilename(t *testing.T) {
	got := DefaultFilename(time.Date(2026, 9, 17, 16, 47, 5, 0, time.UTC))
	want := "palaver-transcript-20260917-164705.txt"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAppendWritesLinesAndSkipsBlank(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	w, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = w.Close() }()

	if w.Path() != path {
		t.Errorf("path: got %q want %q", w.Path(), path)
	}

	if err := w.Append("hello world"); err != nil {
		t.Fatalf("append hello: %v", err)
	}
	if err := w.Append("  "); err != nil {
		t.Fatalf("append blank: %v", err)
	}
	if err := w.Append("[BLANK_AUDIO]"); err != nil {
		t.Fatalf("append blank audio: %v", err)
	}
	if err := w.Append("second line"); err != nil {
		t.Fatalf("append second: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(data)
	want := "hello world\nsecond line\n"
	if got != want {
		t.Errorf("file contents:\n got %q\nwant %q", got, want)
	}
}

func TestOpenAppendsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("already\n"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	w, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := w.Append("more"); err != nil {
		t.Fatalf("append: %v", err)
	}
	_ = w.Close()

	data, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.HasPrefix(string(data), "already\n") || !strings.Contains(string(data), "more\n") {
		t.Errorf("unexpected contents %q", data)
	}
}

func TestAppendAfterClose(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := w.Append("nope"); err == nil {
		t.Error("expected error after close")
	}
}
