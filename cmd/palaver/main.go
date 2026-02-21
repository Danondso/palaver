package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gordonklaus/portaudio"

	"github.com/Danondso/palaver/internal/chime"
	"github.com/Danondso/palaver/internal/config"
	"github.com/Danondso/palaver/internal/hotkey"
	"github.com/Danondso/palaver/internal/recorder"
	"github.com/Danondso/palaver/internal/transcriber"
	"github.com/Danondso/palaver/internal/tui"
)

// micCheckerAdapter adapts the package-level recorder.MicAvailable function
// to the tui.MicChecker interface.
type micCheckerAdapter struct{}

func (micCheckerAdapter) MicAvailable() bool {
	return recorder.MicAvailable()
}

func (micCheckerAdapter) MicName() string {
	return recorder.MicName()
}

func main() {
	debug := flag.Bool("debug", false, "enable debug logging to stderr")
	flag.Parse()

	// Set up debug logger
	var dbg *log.Logger
	if *debug {
		dbg = log.New(os.Stderr, "[DEBUG] ", log.Ltime|log.Lmicroseconds)
	} else {
		dbg = log.New(io.Discard, "", 0)
	}

	// Load config
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Suppress ALSA/JACK noise during PortAudio init by redirecting stderr
	stderrFd := int(os.Stderr.Fd())
	savedStderr, err := syscall.Dup(stderrFd)
	if err != nil {
		log.Fatalf("dup stderr: %v", err)
	}
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		log.Fatalf("open /dev/null: %v", err)
	}
	syscall.Dup2(int(devNull.Fd()), stderrFd)
	devNull.Close()

	if err := portaudio.Initialize(); err != nil {
		// Restore stderr before logging
		syscall.Dup2(savedStderr, stderrFd)
		syscall.Close(savedStderr)
		log.Fatalf("portaudio init: %v", err)
	}
	defer portaudio.Terminate()

	// Restore stderr
	syscall.Dup2(savedStderr, stderrFd)
	syscall.Close(savedStderr)

	dbg.Printf("portaudio initialized")

	// Create transcriber
	trans, err := transcriber.New(&cfg.Transcription, dbg)
	if err != nil {
		log.Fatalf("create transcriber: %v", err)
	}

	// Create chime player
	chimePlayer, err := chime.New(cfg.Audio.ChimeStart, cfg.Audio.ChimeStop, cfg.Audio.ChimeEnabled)
	if err != nil {
		log.Fatalf("create chime player: %v", err)
	}

	// Create recorder
	rec, err := recorder.New(cfg.Audio.TargetSampleRate, cfg.Audio.MaxDurationSec)
	if err != nil {
		log.Fatalf("create recorder: %v", err)
	}

	// Resolve hotkey
	keyCode, err := hotkey.KeyCodeFromName(cfg.Hotkey.Key)
	if err != nil {
		log.Fatalf("resolve hotkey: %v", err)
	}
	dbg.Printf("hotkey: %s (code=%d)", cfg.Hotkey.Key, keyCode)

	// Find keyboard device
	dev, err := hotkey.FindKeyboard(cfg.Hotkey.Device)
	if err != nil {
		log.Fatalf("find keyboard: %v", err)
	}
	dbg.Printf("keyboard device: %s", dev.Path())

	// Create TUI model and program
	model := tui.NewModel(cfg, trans, chimePlayer, rec, micCheckerAdapter{}, dbg, *debug)
	p := tea.NewProgram(model, tea.WithAltScreen())

	// When debug is enabled, redirect logger output into the TUI debug panel
	if *debug {
		dbg.SetOutput(tui.NewLogWriter(p))
	}

	// Hotkey listener
	listener := hotkey.NewListener(dev)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var recMu sync.Mutex

	go func() {
		err := listener.Start(ctx, keyCode,
			// onDown: start recording
			func() {
				dbg.Printf("hotkey down: %s", cfg.Hotkey.Key)
				recMu.Lock()
				defer recMu.Unlock()
				if err := rec.Start(); err != nil {
					dbg.Printf("recorder start error: %v", err)
					return
				}
				p.Send(tui.RecordingStartedMsg{})
			},
			// onUp: stop recording, send WAV data
			func() {
				dbg.Printf("hotkey up: %s", cfg.Hotkey.Key)
				recMu.Lock()
				defer recMu.Unlock()
				wavData, truncated, err := rec.Stop()
				if err != nil {
					dbg.Printf("recorder stop error: %v", err)
					p.Send(tui.TranscriptionErrorMsg{Err: fmt.Errorf("recording: %w", err)})
					return
				}
				dbg.Printf("recording stopped: wav_size=%d bytes, truncated=%v", len(wavData), truncated)
				p.Send(tui.RecordingStoppedMsg{WavData: wavData})
			},
		)
		if err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "hotkey listener error: %v\n", err)
		}
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}

	// Clean shutdown
	cancel()
	listener.Stop()
}
