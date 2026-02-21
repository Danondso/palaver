# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Palaver is a Linux voice-to-text tool written in Go. It listens for a global hotkey (via evdev), records audio (via PortAudio), sends it to a transcription backend (OpenAI-compatible API or shell command), and pastes the result into the active application. Supports X11 (xdotool/xclip) and Wayland/Cosmic (wl-copy/ydotool).

## Build & Test Commands

```bash
# Build
go build -o palaver ./cmd/palaver/

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/recorder

# Run a specific test
go test -run TestResample ./internal/recorder

# Format code
go fmt ./...
```

No Makefile, CI, or linting configuration exists — use the standard Go toolchain directly.

Use `--debug` flag for verbose logging: `./palaver --debug`

## System Dependencies

Build: `libportaudio2`, `portaudio19-dev`
Runtime (X11): `xdotool`, Linux evdev access (user in `input` group)
Runtime (Wayland/Cosmic): `wl-clipboard`, `ydotool`, Linux evdev access (user in `input` group)

## Architecture

**Entry point:** `cmd/palaver/main.go` — loads config, initializes all components, starts the hotkey listener in a goroutine, and runs the Bubble Tea TUI.

**Internal packages** (`internal/`):

| Package | Role |
|---------|------|
| `config` | TOML config from `~/.config/palaver/config.toml` with built-in defaults |
| `hotkey` | Global hotkey via Linux evdev; auto-detects keyboard device from `/dev/input/event*` |
| `recorder` | PortAudio capture → polyphase FIR resampling (48/44.1kHz → 16kHz) → WAV encoding (mono 16-bit PCM) |
| `transcriber` | `Transcriber` interface with two providers: `openai` (HTTP multipart to `/v1/audio/transcriptions`) and `command` (shell out with `{input}` template) |
| `tui` | Bubble Tea state machine: Idle → Recording → Transcribing → Idle (+ Error with 5s auto-clear); configurable themes (synthwave, everforest, gruvbox, monochrome) |
| `clipboard` | Paste: auto-detects X11 vs Wayland; default "type" mode uses xdotool/ydotool direct typing; "clipboard" mode uses clipboard+Ctrl+V (auto-starts ydotoold) |
| `chime` | Embedded start/stop WAV chimes played via beep library; customizable paths in config |

**Data flow:** Hotkey press → record audio → resample → encode WAV → transcribe → paste text

**Key patterns:**
- Bubble Tea message-passing for async operations (RecordingStartedMsg, RecordingStoppedMsg, TranscriptionResultMsg, TranscriptionErrorMsg)
- Interface-based transcription (`transcriber.Transcriber`) with factory function `New(cfg, logger)`
- Mutex-protected recording state for thread safety
- Embedded WAV assets compiled into the binary (`chime/` package uses `//go:embed`)
- Default transcription endpoint: `http://localhost:5092` (NVIDIA Parakeet / faster-whisper-server)
- Default transcription model: `whisper-1`
- Default max recording duration: 60 seconds
- Default paste mode: `type` (direct typing); alternative `clipboard` mode uses Ctrl+V
- Theme system: 4 built-in themes selectable via config or `t` key at runtime
