# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Palaver is a cross-platform (Linux and macOS) voice-to-text tool written in Go. It listens for a global hotkey, records audio (via PortAudio), sends it to a transcription backend (OpenAI-compatible API or shell command), and pastes the result into the active application.

Platform support:
- **Linux:** Hotkey via evdev, paste via xdotool/xclip (X11) or wl-clipboard/ydotool (Wayland/Cosmic), managed Parakeet server with ONNX Runtime
- **macOS:** Hotkey via CGEventTap (CGO), paste via pbcopy/osascript, managed whisper-cpp server (via Homebrew)

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

No Makefile exists — use the standard Go toolchain directly. CI workflows (`.github/workflows/`) and a `.golangci.yml` lint config are present.

Use `--debug` flag for verbose logging: `./palaver --debug`

Use `palaver setup` to download the managed server and model files (Parakeet + ONNX on Linux, whisper model on macOS).

## System Dependencies

**Linux:**
- Build: `libportaudio2`, `portaudio19-dev`
- Runtime (X11): `xdotool`, evdev access (user in `input` group)
- Runtime (Wayland/Cosmic): `wl-clipboard`, `ydotool`, evdev access (user in `input` group)

**macOS:**
- Build/Runtime: `portaudio`, `whisper-cpp` (via Homebrew)
- Permissions: Accessibility (System Settings > Privacy & Security), Input Monitoring

## Architecture

**Entry point:** `cmd/palaver/main.go` — loads config, initializes all components, starts the hotkey listener in a goroutine, and runs the Bubble Tea TUI. Platform-specific entry in `entry_linux.go` / `entry_darwin.go`, hotkey wiring in `hotkey_linux.go` / `hotkey_darwin.go`.

**Internal packages** (`internal/`):

| Package | Role |
|---------|------|
| `config` | TOML config from `~/.config/palaver/config.toml` with built-in defaults; platform-specific defaults in `defaults_linux.go` / `defaults_darwin.go` |
| `hotkey` | Global hotkey listener. Linux: evdev, auto-detects keyboard from `/dev/input/event*`. macOS: CGEventTap via CGO (`cgeventtap_darwin.c`), supports modifier+key and modifier-only combos |
| `recorder` | PortAudio capture → polyphase FIR resampling (48/44.1kHz → 16kHz) → WAV encoding (mono 16-bit PCM) |
| `transcriber` | `Transcriber` interface with two providers: `openai` (HTTP multipart to `/v1/audio/transcriptions`) and `command` (shell out with `{input}` template) |
| `tui` | Bubble Tea state machine: Idle → Recording → Transcribing → [PostProcessing →] Pasting → Idle (+ Error with 5s auto-clear); configurable themes (synthwave, everforest, gruvbox, monochrome) |
| `postprocess` | LLM-based text rewriting via OpenAI-compatible chat completions API (Ollama default); built-in tone presets (formal, direct, token-efficient) with custom tone support via config |
| `clipboard` | Paste text into active application. Linux: auto-detects X11 vs Wayland; only "type" mode works (xdotool on X11, ydotool on Wayland); "clipboard" mode is broken on Linux. macOS: "clipboard" mode (default) uses pbcopy+Cmd+V via osascript, "type" mode uses osascript keystroke |
| `chime` | Embedded start/stop WAV chimes played via beep library; customizable paths in config |
| `server` | Managed transcription server lifecycle. Linux: Parakeet (download binary, ONNX Runtime, model files). macOS: whisper-cpp/whisper-server (via Homebrew, downloads ggml model). Both: start/stop/restart, auto-start on launch |

**Data flow:** Hotkey press → record audio → resample → encode WAV → transcribe → [post-process via LLM →] paste text

**Key patterns:**
- Bubble Tea message-passing for async operations (RecordingStartedMsg, RecordingStoppedMsg, TranscriptionResultMsg, TranscriptionErrorMsg)
- Interface-based transcription (`transcriber.Transcriber`) with factory function `New(cfg, logger)`
- Mutex-protected recording state for thread safety
- Embedded WAV assets compiled into the binary (`chime/` package uses `//go:embed`)
- Platform-specific code uses Go build tags (`//go:build linux` / `//go:build darwin`)
- Default transcription endpoint: `http://localhost:5092`
- Default transcription model: `whisper-1`
- Default max recording duration: 60 seconds
- Default hotkey: `KEY_RIGHTCTRL` (Linux), `Cmd+Option` (macOS)
- Default paste mode: `type` (Linux), `clipboard` (macOS)
- Theme system: 4 built-in themes selectable via config or `t` key at runtime
- Post-processing: optional LLM rewriting of transcribed text; `p` key cycles tones, `m` key cycles models; defaults to Ollama at `http://localhost:11434/v1` with `llama3.2` model, disabled by default
- Managed server: auto-start on launch; `r` key to restart; `palaver setup` to install
