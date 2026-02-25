package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gordonklaus/portaudio"

	"github.com/Danondso/palaver/internal/chime"
	"github.com/Danondso/palaver/internal/config"
	"github.com/Danondso/palaver/internal/postprocess"
	"github.com/Danondso/palaver/internal/recorder"
	"github.com/Danondso/palaver/internal/server"
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

func handleSetup() {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	dbg := log.New(os.Stderr, "[SETUP] ", log.Ltime)
	runSetup(cfg, dbg)
}

func runSetup(cfg *config.Config, dbg *log.Logger) {
	srv := server.New(&cfg.Server, dbg)

	fmt.Println("=== Palaver Setup ===")
	fmt.Println()

	progress := func(stage string, downloaded, total int64) {
		if total > 0 {
			pct := float64(downloaded) / float64(total) * 100
			fmt.Printf("\r  [%s] %.1f%% (%d / %d bytes)", stage, pct, downloaded, total)
		} else {
			fmt.Printf("\r  [%s] %d bytes", stage, downloaded)
		}
	}

	if err := srv.Setup(progress); err != nil {
		fmt.Printf("\nSetup failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println()

	// Verify the server starts (only if it was installed)
	if srv.IsInstalled() {
		fmt.Println("Starting server to verify installation...")
		ctx, cancel := context.WithCancel(context.Background())
		if err := srv.Start(ctx); err != nil {
			cancel()
			fmt.Printf("Server failed to start: %v\n", err)
			fmt.Println("Check the error above. If ONNX Runtime is still missing, the download may have failed.")
			os.Exit(1)
		}
		fmt.Println("Server is healthy!")
		srv.Stop()
		cancel()
	}

	fmt.Println()
	fmt.Println("Setup complete. Run 'palaver' to start.")
}

func run() {
	// Handle setup subcommand before flag parsing
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		handleSetup()
		return
	}

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

	// Initialize PortAudio (Linux suppresses ALSA/JACK stderr noise)
	if err := initPortAudio(); err != nil {
		log.Fatalf("portaudio init: %v", err)
	}
	defer portaudio.Terminate()

	dbg.Printf("portaudio initialized")

	// Create transcriber
	trans, err := transcriber.New(&cfg.Transcription, dbg)
	if err != nil {
		log.Fatalf("create transcriber: %v", err)
	}

	// Create post-processor
	pp := postprocess.New(&cfg.PostProcessing, cfg.CustomTones, dbg)

	// Warn if sending audio over plaintext HTTP to a non-local host
	if u, err := url.Parse(cfg.Transcription.BaseURL); err == nil {
		if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
			log.Printf("WARNING: transcription base_url uses plaintext HTTP to non-local host %q — audio data will be sent unencrypted", u.Hostname())
		}
	}

	// Warn if sending transcribed text over plaintext HTTP to a non-local host
	if cfg.PostProcessing.Enabled {
		if u, err := url.Parse(cfg.PostProcessing.BaseURL); err == nil {
			if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
				log.Printf("WARNING: post_processing base_url uses plaintext HTTP to non-local host %q — transcribed text will be sent unencrypted", u.Hostname()) //nolint:gosec // hostname from user config, safely quoted with %q
			}
		}
	}

	// Create chime player
	chimePlayer, err := chime.New(cfg.Audio.ChimeStart, cfg.Audio.ChimeStop, cfg.Audio.ChimeEnabled, dbg)
	if err != nil {
		log.Fatalf("create chime player: %v", err)
	}

	// Create recorder
	rec, err := recorder.New(cfg.Audio.TargetSampleRate, cfg.Audio.MaxDurationSec)
	if err != nil {
		log.Fatalf("create recorder: %v", err)
	}

	// Create hotkey listener (platform-specific)
	listener, err := createListener(cfg, dbg)
	if err != nil {
		log.Fatalf("create hotkey listener: %v", err)
	}
	dbg.Printf("hotkey: %s", listener.KeyName())

	// Managed server (auto-start if configured and installed)
	var srv *server.Server
	if cfg.Server.AutoStart {
		srv = server.New(&cfg.Server, dbg)
		if srv.IsInstalled() {
			dbg.Printf("managed parakeet server is installed, will auto-start")
		} else {
			dbg.Printf("managed server not installed (run 'palaver setup' first)")
			srv = nil
		}
	}

	// Create TUI model and program
	model := tui.NewModel(cfg, trans, pp, chimePlayer, rec, micCheckerAdapter{}, dbg, *debug)
	model.Server = srv
	serverCtx, serverCancel := context.WithCancel(context.Background())
	model.ServerCtx = serverCtx
	model.ServerCancel = serverCancel
	p := tea.NewProgram(model, tea.WithAltScreen())

	// When debug is enabled, redirect logger output into the TUI debug panel
	if *debug {
		dbg.SetOutput(tui.NewLogWriter(p))
	}

	// Hotkey listener
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var recMu sync.Mutex

	go func() {
		err := listener.Start(ctx,
			// onDown: start recording
			func() {
				dbg.Printf("hotkey down: %s", listener.KeyName())
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
				dbg.Printf("hotkey up: %s", listener.KeyName())
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
	serverCancel()
	if srv != nil {
		srv.Stop()
	}
}
