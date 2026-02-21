## Relevant Files

- `cmd/palaver/main.go` - Application entry point, config loading, dependency wiring, starts Bubble Tea program
- `internal/config/config.go` - TOML config struct, defaults, loading from `~/.config/palaver/config.toml`
- `internal/config/config_test.go` - Unit tests for config loading, defaults, overrides
- `internal/hotkey/hotkey.go` - Global evdev hotkey listener, keyboard auto-detection, key-down/up events
- `internal/hotkey/hotkey_test.go` - Unit tests for hotkey parsing, device detection logic
- `internal/recorder/recorder.go` - PortAudio microphone capture, PCM buffer, stereo-to-mono downmix, resampling, WAV encoding
- `internal/recorder/recorder_test.go` - Unit tests for WAV encoding, resampling, downmix logic
- `internal/transcriber/transcriber.go` - `Transcriber` interface definition
- `internal/transcriber/openai.go` - OpenAI-compatible API provider (multipart POST to `/v1/audio/transcriptions`)
- `internal/transcriber/openai_test.go` - Unit tests for OpenAI provider (mock HTTP server)
- `internal/transcriber/command.go` - Command provider (shell out to external tool)
- `internal/transcriber/command_test.go` - Unit tests for command provider
- `internal/clipboard/clipboard.go` - Write text to clipboard via xclip + simulate Ctrl+V via xdotool
- `internal/clipboard/clipboard_test.go` - Unit tests for clipboard module
- `internal/chime/chime.go` - Load and play start/stop chime sounds (embedded defaults + custom paths)
- `internal/chime/assets/start.wav` - Embedded default start chime
- `internal/chime/assets/stop.wav` - Embedded default stop chime
- `internal/chime/chime_test.go` - Unit tests for chime loading logic
- `internal/tui/model.go` - Bubble Tea model struct, message types, state enum
- `internal/tui/update.go` - Bubble Tea Update function, state machine transitions, dispatches commands
- `internal/tui/view.go` - Bubble Tea View function, Lip Gloss 80s Miami styling, layout rendering
- `internal/tui/tui_test.go` - Unit tests for state transitions and update logic
- `go.mod` - Go module definition and dependencies
- `README.md` - Project documentation, setup instructions, backend configuration examples

### Notes

- Unit tests should be placed alongside the code files they test (e.g., `config.go` and `config_test.go` in the same directory).
- Use `go test ./...` to run all tests. Use `go test ./internal/recorder/` to run tests for a specific package.
- This project requires CGO for PortAudio bindings. Ensure `libportaudio2` and `portaudio19-dev` are installed.
- Reading `/dev/input/event*` requires the user to be in the `input` group: `sudo usermod -aG input $USER`

## Instructions for Completing Tasks

**IMPORTANT:** As you complete each task, you must check it off in this markdown file by changing `- [ ]` to `- [x]`. This helps track progress and ensures you don't skip any steps.

Example:
- `- [ ] 1.1 Read file` → `- [x] 1.1 Read file` (after completing)

Update the file after completing each sub-task, not just after completing an entire parent task.

## Tasks

- [x] 0.0 Project initialization and Go module setup
  - [x] 0.1 Run `go mod init github.com/Danondso/palaver` in the project root
  - [x] 0.2 Create the directory structure: `cmd/palaver/`, `internal/config/`, `internal/hotkey/`, `internal/recorder/`, `internal/transcriber/`, `internal/clipboard/`, `internal/chime/assets/`, `internal/tui/`
  - [x] 0.3 Add core dependencies: `go get github.com/charmbracelet/bubbletea github.com/charmbracelet/lipgloss github.com/BurntSushi/toml github.com/gordonklaus/portaudio github.com/go-audio/wav github.com/go-audio/audio github.com/gopxl/beep github.com/holoplot/go-evdev github.com/atotto/clipboard github.com/zaf/resample`
  - [x] 0.4 Add a placeholder `cmd/palaver/main.go` with `package main` and an empty `func main()` that compiles successfully
  - [x] 0.5 Verify `go build ./cmd/palaver/` compiles without errors
- [x] 1.0 Configuration system (`internal/config`)
  - [x] 1.1 Define the `Config` struct in `internal/config/config.go` with nested structs matching the TOML schema: `HotkeyConfig` (key `KEY_RIGHTCTRL`, device), `AudioConfig` (target_sample_rate 16000, max_duration_sec 30, chime_start, chime_stop, chime_enabled true), `TranscriptionConfig` (provider `openai`, base_url `http://localhost:5092`, model `default`, timeout_sec 30, command), `PasteConfig` (delay_ms 50)
  - [x] 1.2 Implement `Load(path string) (*Config, error)` that reads and parses the TOML file, falling back to defaults if the file does not exist
  - [x] 1.3 Implement `Default() *Config` that returns a fully populated config with all default values
  - [x] 1.4 Write unit tests: verify defaults are correct, verify TOML overrides work for each field, verify missing file returns defaults without error
- [x] 2.0 Global hotkey listener (`internal/hotkey`)
  - [x] 2.1 Implement `FindKeyboard(devicePath string) (*evdev.InputDevice, error)`
  - [x] 2.2 Implement `KeyCodeFromName(name string) (evdev.EvCode, error)`
  - [x] 2.3 Implement `type Listener struct` with `Start()` and event loop
  - [x] 2.4 Implement `Stop()` method
  - [x] 2.5 Write unit tests for `KeyCodeFromName` mapping
- [x] 3.0 Audio capture, resampling, and WAV encoding (`internal/recorder`)
  - [x] 3.1 Implement `type Recorder struct` with PortAudio, native sample rate query
  - [x] 3.2 Implement `Start()` with PCM buffer capture
  - [x] 3.3 Implement stereo-to-mono downmix
  - [x] 3.4 Implement `Stop()` with polyphase FIR resampling (`go-audio-resampling`) and WAV encoding
  - [x] 3.5 Implement max duration enforcement
  - [x] 3.6 Write unit tests (7 tests passing)
- [x] 4.0 Transcription provider interface and implementations (`internal/transcriber`)
  - [x] 4.1 Define `Transcriber` interface + factory function
  - [x] 4.2 Implement `OpenAI` provider (multipart POST)
  - [x] 4.3 Implement `Command` provider (shell out)
  - [x] 4.4-4.6 Unit tests (6 tests passing)
- [x] 5.0 Clipboard paste module (`internal/clipboard`)
  - [x] 5.1 Implement `PasteText` with xclip + xdotool
  - [x] 5.2 Error handling for missing xdotool
  - [x] 5.3 Tests
- [x] 6.0 Audio chime feedback (`internal/chime`)
  - [x] 6.1 Generated start/stop WAV chimes (sine wave tones)
  - [x] 6.2 Embedded via `//go:embed`
  - [x] 6.3-6.5 Player with beep speaker, custom paths, disabled mode
  - [x] 6.6 Unit tests (5 tests passing)
- [x] 7.0 Bubble Tea TUI with 80s Miami styling (`internal/tui`)
  - [x] 7.1-7.4 Model, state enum, message types, Init()
  - [x] 7.5 Update() with full state machine
  - [x] 7.6 View() with Miami synthwave Lip Gloss styling
  - [x] 7.7 Unit tests (9 tests passing)
- [x] 8.0 Application entry point and wiring (`cmd/palaver/main.go`)
  - [x] 8.1-8.5 Config loading, dependency init, TUI program, hotkey goroutine, clean shutdown
  - [x] 8.6 Build verified — compiles successfully
- [ ] 9.0 End-to-end integration testing and README documentation
  - [x] 9.1 `go test ./...` — all 35 tests pass
  - [ ] 9.2 Manual integration test (requires transcription backend + display server)
  - [ ] 9.3 Test custom config
  - [ ] 9.4 Test chime customization
  - [ ] 9.5 Test error handling
  - [x] 9.6 Write README.md
  - [x] 9.7 `go vet ./...` and build — clean
