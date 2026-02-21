# Palaver — Product Requirements Document (PRD)

## 1. Introduction / Overview

Palaver is a minimal, privacy-first, voice-to-text transcription tool for Linux desktops. It runs as a terminal user interface (TUI) built in Go with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework. The user holds a configurable hotkey (default: F12), speaks into their microphone, and upon release the recorded audio is sent to a local transcription server for speech-to-text conversion. The resulting text is placed on the system clipboard and automatically pasted into the currently focused application via a simulated `Ctrl+V` keystroke.

Palaver is conceptually similar to [Vocalinux](https://github.com/jatinkrmalik/vocalinux) and inspired by [VoiceInk](https://github.com/Beingpax/VoiceInk), but differs in several important ways:

- Written in **Go** rather than Python/Swift
- Uses a **TUI** rather than a GTK GUI or native macOS UI
- Uses a **pluggable HTTP-based transcription backend** rather than linking whisper.cpp or CoreML directly
- Uses **clipboard-based pasting** for simplicity
- Runs entirely on **Linux**

### Note on Transcription Backend

Ollama does not currently have a native audio transcription API endpoint ([issue #11798](https://github.com/ollama/ollama/issues/11798), [issue #8202](https://github.com/ollama/ollama/issues/8202)). The PRD defines a **Transcription Provider Interface** that communicates with any server implementing the OpenAI-compatible `POST /v1/audio/transcriptions` endpoint. This covers multiple backends — see [Section 7.3: Recommended Transcription Backends](#73-recommended-transcription-backends) for setup examples. The day Ollama ships audio support, users simply point Palaver at their Ollama instance with zero code changes.

---

## 2. Goals

| # | Goal |
|---|------|
| G1 | Provide a single-binary, zero-daemon voice-to-text tool for Linux that works across all desktop applications. |
| G2 | Keep the core workflow under three seconds end-to-end for a typical five-second utterance on local hardware. |
| G3 | Require minimal system dependencies (PortAudio, xdotool, xclip). |
| G4 | Make the transcription backend pluggable so it is not coupled to any single speech model or API. |
| G5 | Provide clear audio and visual feedback so the user always knows whether Palaver is idle, recording, or transcribing. |
| G6 | All processing happens locally by default — no data leaves the user's machine. |

---

## 3. User Stories

| ID | As a... | I want to... | So that... |
|----|---------|-------------|------------|
| US1 | Linux desktop user | Hold F12, speak, and release to have my words typed into the active text field | I can dictate text without a keyboard. |
| US2 | User | See a TUI status indicator showing idle / recording / transcribing / error | I know the current state at a glance. |
| US3 | User | Hear a short chime when recording starts and stops | I get immediate audio confirmation without looking at my terminal. |
| US4 | Power user | Change the hotkey, transcription backend URL, model name, and audio chime files via a config file | I can customize Palaver to my setup. |
| US5 | User | See the last transcribed text in the TUI | I can verify what was recognized before it is pasted. |
| US6 | User | Press `q` or `Ctrl+C` in the TUI to quit cleanly | The program exits gracefully, releasing the microphone and hotkey. |

---

## 4. Functional Requirements

### 4.1 Global Hotkey Listener

| # | Requirement |
|---|-------------|
| FR1 | The application MUST listen for a global push-to-talk hotkey even when the TUI terminal is not focused. |
| FR2 | The default hotkey MUST be KEY_RIGHTCTRL. The user MUST be able to override this in the config file using a Linux evdev key name (e.g., `KEY_F12`, `KEY_RIGHTCTRL`). |
| FR3 | On key-down, the application MUST begin capturing audio from the default microphone and transition to the "recording" state. |
| FR4 | On key-up, the application MUST stop capturing audio and transition to the "transcribing" state. |
| FR5 | The hotkey listener MUST use the Linux evdev input subsystem (`/dev/input/event*`) so that it works on both X11 and Wayland. |
| FR6 | The application MUST auto-detect the keyboard input device or allow the user to specify it in config. |

### 4.2 Audio Capture

| # | Requirement |
|---|-------------|
| FR7 | Audio MUST be captured via PortAudio at the device's native sample rate (typically 44100 Hz or 48000 Hz). The application MUST query the default input device for its preferred sample rate rather than assuming 16 kHz. |
| FR8 | Raw PCM samples MUST be accumulated in a memory buffer during recording (16-bit, mono). If the device captures in stereo, the application MUST downmix to mono by averaging channels. |
| FR9 | When recording stops, the buffer MUST be resampled to 16 kHz (the format expected by Whisper-family and Parakeet models) if the capture rate differs from 16 kHz. Resampling MUST use a high-quality algorithm (e.g., linear interpolation at minimum; ideally sinc/polyphase via a library like `github.com/zaf/resample` or `github.com/mjibson/go-dsp`). |
| FR10 | After resampling, the buffer MUST be encoded to WAV format (16 kHz, 16-bit, mono PCM) in memory (no temporary files on disk). |
| FR11 | If recording exceeds 30 seconds, the application MUST automatically stop recording and proceed to transcription, displaying a warning in the TUI. |
| FR12 | The target sample rate (default 16000) MUST be configurable in case a backend expects a different rate. |

### 4.3 Transcription

| # | Requirement |
|---|-------------|
| FR13 | The application MUST define a `Transcriber` interface: `Transcribe(ctx context.Context, wavData []byte) (string, error)`. |
| FR14 | The **OpenAI-compatible provider** MUST send a `multipart/form-data` POST request to `{base_url}/v1/audio/transcriptions` with fields `file` (the WAV bytes), `model` (configurable), and `response_format=text`. |
| FR15 | The **Command provider** MUST write the WAV data to a temporary file, execute the configured command with `{input}` replaced by the temp file path, read stdout as the transcript, and delete the temp file. |
| FR16 | On success, the application MUST transition to "idle" and display the transcript in the TUI. |
| FR17 | On failure (network error, non-200 status, timeout), the application MUST transition to "error," display the error in the TUI, and return to "idle" after 5 seconds. |
| FR18 | Transcription requests MUST have a configurable timeout (default: 30 seconds). |

### 4.4 Clipboard Paste

| # | Requirement |
|---|-------------|
| FR19 | On successful transcription, the text MUST be written to the system clipboard. |
| FR20 | After writing to the clipboard, the application MUST simulate a `Ctrl+V` keystroke using `xdotool`. |
| FR21 | There MUST be a configurable delay (default: 50 ms) between clipboard write and keystroke simulation to account for slow clipboard managers. |

### 4.5 Audio Feedback (Chimes)

| # | Requirement |
|---|-------------|
| FR22 | A short audio chime MUST play when recording starts and a different chime when recording stops. |
| FR23 | The user MUST be able to specify custom WAV file paths for start and stop chimes in the config, or set them to empty string to disable. |
| FR24 | Default chimes MUST be embedded in the binary using Go's `embed` package. |
| FR25 | Chime playback MUST be non-blocking (fire-and-forget in a goroutine). |

### 4.6 TUI

| # | Requirement |
|---|-------------|
| FR26 | The TUI MUST display: (a) current state as a colored status badge (green=idle, red=recording, yellow=transcribing, red=error), (b) the last transcription result, (c) a help line showing the configured hotkey and quit instructions. |
| FR27 | The TUI MUST accept `q` and `Ctrl+C` to quit. |
| FR28 | The TUI MUST be styled with Lip Gloss. |

### 4.7 Configuration

| # | Requirement |
|---|-------------|
| FR29 | Configuration MUST be loaded from `~/.config/palaver/config.toml`. If the file does not exist, defaults MUST be used. |
| FR30 | The config file MUST support the following keys with these defaults: |

```toml
[hotkey]
key = "KEY_RIGHTCTRL"    # evdev key name
device = ""              # empty = auto-detect keyboard

[audio]
target_sample_rate = 16000  # resample to this rate before sending to backend
max_duration_sec = 30
chime_start = ""         # empty = use embedded default; path to custom WAV
chime_stop = ""          # empty = use embedded default; path to custom WAV
chime_enabled = true

[transcription]
provider = "openai"      # "openai" or "command"
base_url = "http://localhost:5092"  # for openai provider (default = parakeet server)
model = "default"
timeout_sec = 30
command = ""             # for command provider, e.g. "whisper-cpp -m base.en -f {input}"

[paste]
delay_ms = 50
```

---

## 5. Non-Goals (Out of Scope for v1)

- **Streaming / real-time transcription** — v1 is batch-only (record, then transcribe).
- **Multiple language selection UI** — the underlying model handles languages; Palaver does not expose language selection.
- **System tray icon / desktop notifications** — TUI only.
- **Wayland-native paste** — v1 uses `xdotool` which requires XWayland. Native Wayland paste (via `wtype` or `ydotool`) is a future enhancement.
- **Audio preprocessing** — no noise reduction, VAD, or silence trimming.
- **Automatic backend installation** — the user is responsible for running their own transcription server.
- **macOS or Windows support** — Linux only.
- **GUI settings dialog** — configuration is file-based only.

---

## 6. Design Considerations (TUI Layout)

The TUI is intentionally minimal. It is a single-screen status display, not an interactive form. The visual style draws from an **80s Miami / synthwave** color palette.

### Color Palette (Lip Gloss)

| Role | Color | Hex | Usage |
|------|-------|-----|-------|
| Hot Pink | Neon magenta/pink | `#FF6AC1` | Title "PALAVER", recording state badge |
| Cyan | Electric cyan | `#00E5FF` | Borders, status label text, hotkey display |
| Purple | Deep violet | `#B388FF` | Transcription result text |
| Peach/Coral | Warm coral | `#FF8A80` | Error state badge, warnings |
| Teal | Miami teal | `#64FFDA` | Idle state badge, success indicators |
| Sunset Orange | Warm orange | `#FFAB40` | Transcribing state badge |
| Background | Dark navy/charcoal | `#1A1A2E` | Main background (if terminal supports it) |
| Foreground | Soft white | `#E0E0E0` | Default body text |

### Layout

```
┌─────────────────────────────────────────────┐  ← cyan border
│                                             │
│   ▓▓▓  PALAVER  ▓▓▓                        │  ← hot pink title
│                                             │
│   Status:  ● Idle                           │  ← "Status:" in cyan, badge in teal
│                                             │
│   Last transcription:                       │  ← label in cyan
│   "Hello, this is a test of palaver."       │  ← text in purple
│                                             │
│   Hotkey: RIGHT CTRL (hold to record)       │  ← cyan
│   Press q to quit                           │  ← dimmed/muted
│                                             │
└─────────────────────────────────────────────┘  ← cyan border
```

### State Badge Colors

| State | Badge Color | Badge Text |
|-------|------------|------------|
| Idle | Teal (`#64FFDA`) | `● Idle` |
| Recording | Hot Pink (`#FF6AC1`) pulsing | `● Recording...` |
| Transcribing | Sunset Orange (`#FFAB40`) | `● Transcribing...` |
| Error | Coral (`#FF8A80`) | `● Error: <message>` |

**Bubble Tea architecture:**

- **Model** (`model` struct): holds `state` (enum), `lastTranscript` (string), `lastError` (string), `config` (parsed config), `audioBuffer` (reference to the recording subsystem).
- **Messages** (custom `tea.Msg` types):
  - `recordingStartedMsg` — hotkey pressed, recording began
  - `recordingStoppedMsg{wavData []byte}` — hotkey released, WAV data ready
  - `transcriptionResultMsg{text string}` — transcription succeeded
  - `transcriptionErrorMsg{err error}` — transcription failed
  - `errorTimeoutMsg` — 5 seconds elapsed after error, return to idle
- **Update**: handles each message, transitions state, dispatches `tea.Cmd` for async work (transcription HTTP call, error timeout).
- **View**: renders the status screen using Lip Gloss styles.

The hotkey listener and audio recorder run in separate goroutines and communicate with the Bubble Tea program via `program.Send(msg)`.

---

## 7. Technical Considerations

### 7.1 Go Packages

| Concern | Package | Notes |
|---------|---------|-------|
| TUI framework | [`github.com/charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) | Elm architecture for terminal UIs. |
| TUI styling | [`github.com/charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) | Declarative terminal styling. |
| Audio capture | [`github.com/gordonklaus/portaudio`](https://github.com/gordonklaus/portaudio) | Go bindings for PortAudio. CGO required. |
| WAV encoding | [`github.com/go-audio/wav`](https://github.com/go-audio/wav) + [`github.com/go-audio/audio`](https://github.com/go-audio/audio) | Pure Go WAV encoder/decoder. |
| Audio resampling | [`github.com/zaf/resample`](https://github.com/zaf/resample) or [`github.com/mjibson/go-dsp`](https://github.com/mjibson/go-dsp) | Resample from device native rate (e.g., 44.1/48 kHz) to 16 kHz for transcription. |
| Chime playback | [`github.com/gopxl/beep`](https://github.com/gopxl/beep) | WAV decode + speaker output. Uses PortAudio under the hood. |
| Global hotkey (evdev) | [`github.com/holoplot/go-evdev`](https://github.com/holoplot/go-evdev) | Pure Go, no CGO. Reads `/dev/input/event*`. |
| Clipboard | [`github.com/atotto/clipboard`](https://github.com/atotto/clipboard) | Wraps `xclip`/`xsel`. Text-only. |
| Config parsing | [`github.com/BurntSushi/toml`](https://github.com/BurntSushi/toml) | TOML parser for Go. |
| HTTP client | `net/http` (stdlib) | For OpenAI-compatible API calls. |
| Embed assets | `embed` (stdlib) | Embed default chime WAV files in the binary. |
| Keystroke simulation | `os/exec` calling `xdotool key ctrl+v` | External dependency, not a Go library. |

### 7.2 System Dependencies

The user must have these installed on their Linux system:

| Dependency | Debian/Ubuntu package | Purpose |
|------------|----------------------|---------|
| PortAudio dev headers | `libportaudio2 portaudio19-dev` | Required at compile time for `gordonklaus/portaudio`. |
| xdotool | `xdotool` | Simulate Ctrl+V keystroke. |
| xclip or xsel | `xclip` | Clipboard access (used by `atotto/clipboard`). |
| A running transcription server | (see Section 7.3) | Local speech-to-text backend. |

### 7.3 Recommended Transcription Backends

Palaver works with any server that implements the OpenAI-compatible `POST /v1/audio/transcriptions` endpoint. Below are recommended backends, ordered by preference:

#### Option A: NVIDIA Parakeet via `achetronic/parakeet` (Recommended)

[Parakeet ASR Server](https://github.com/achetronic/parakeet) — runs the NVIDIA Parakeet TDT 0.6B model via ONNX with CPU-only inference. This is a **drop-in replacement** for OpenAI's Whisper API. Parakeet TDT achieves **3.97% WER** and is significantly faster than Whisper on CPU.

**Install & run:**
```bash
# Download binary
curl -L -o parakeet https://github.com/achetronic/parakeet/releases/latest/download/parakeet-linux-amd64
chmod +x parakeet

# Download models (~670MB)
make models

# Start server (default port 5092)
./parakeet
```

**Or with Docker:**
```bash
docker run -d -p 5092:5092 -v $(pwd)/models:/models ghcr.io/achetronic/parakeet:latest
```

**Test it:**
```bash
curl -X POST http://localhost:5092/v1/audio/transcriptions \
  -F file=@audio.wav \
  -F language=en \
  -F response_format=text
```

Palaver default config points to `http://localhost:5092` — works out of the box with Parakeet.

> **Why Parakeet?** VoiceInk (a popular macOS voice-to-text app) uses NVIDIA's Parakeet TDT models via [FluidAudio](https://github.com/FluidInference/FluidAudio) for their speed and accuracy. Parakeet TDT v3 processes 1 hour of audio in ~19 seconds and supports 25 European languages. The `achetronic/parakeet` server brings this same model to Linux with an OpenAI-compatible API.
>
> Sources: [NVIDIA Parakeet TDT 0.6B v3 (HuggingFace)](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3), [VoiceInk](https://github.com/Beingpax/VoiceInk), [FluidAudio](https://github.com/FluidInference/FluidAudio)

#### Option B: faster-whisper-server (Speaches)

[Speaches](https://github.com/speaches-ai/speaches) — OpenAI-compatible server using faster-whisper (CTranslate2-optimized Whisper). Supports GPU acceleration.

```bash
# CPU
docker run -p 8000:8000 ghcr.io/speaches-ai/speaches:latest

# GPU (NVIDIA)
docker run --gpus=all -p 8000:8000 ghcr.io/speaches-ai/speaches:latest-cuda
```

Palaver config:
```toml
[transcription]
base_url = "http://localhost:8000"
model = "Systran/faster-whisper-base.en"
```

> Source: [faster-whisper](https://github.com/SYSTRAN/faster-whisper), [Speaches](https://github.com/speaches-ai/speaches)

#### Option C: whisper.cpp server

[whisper.cpp](https://github.com/ggml-org/whisper.cpp) — C++ port of OpenAI Whisper with an HTTP server mode.

```bash
git clone https://github.com/ggml-org/whisper.cpp.git
cd whisper.cpp
make -j
./models/download-ggml-model.sh base.en
./build/bin/server -m models/ggml-base.en.bin --port 8080
```

Palaver config:
```toml
[transcription]
base_url = "http://localhost:8080"
model = "whisper-1"
```

> Source: [whisper.cpp](https://github.com/ggml-org/whisper.cpp)

#### Option D: Cloud APIs (Optional, not local)

For users who prefer cloud speed/accuracy over local privacy:

- **Groq** — extremely fast Whisper inference: `base_url = "https://api.groq.com/openai"` (requires API key)
- **OpenAI** — `base_url = "https://api.openai.com"` (requires API key)

#### Future: Ollama

When Ollama adds audio transcription support ([tracking issue](https://github.com/ollama/ollama/issues/11798)), users will simply update their config:

```toml
[transcription]
base_url = "http://localhost:11434"
model = "whisper"
```

No code changes required.

### 7.4 Project Structure

```
palaver/
  cmd/
    palaver/
      main.go              # Entry point, config loading, wiring
  internal/
    config/
      config.go            # TOML config struct, defaults, loading
    hotkey/
      hotkey.go            # evdev listener, key-down/up detection, device auto-detect
    recorder/
      recorder.go          # PortAudio capture, PCM buffer, WAV encoding
    transcriber/
      transcriber.go       # Transcriber interface definition
      openai.go            # OpenAI-compatible API implementation
      command.go           # Command-line shelling implementation
    clipboard/
      clipboard.go         # Write to clipboard + xdotool paste
    chime/
      chime.go             # Load and play chime sounds
      assets/
        start.wav          # Embedded default start chime
        stop.wav           # Embedded default stop chime
    tui/
      model.go             # Bubble Tea model, messages, state machine
      view.go              # Lip Gloss styled view rendering
      update.go            # Update function handling all messages
  go.mod
  go.sum
  README.md
  LICENSE
```

### 7.5 Data Flow

```
[User holds F12]
    │
    ▼
[hotkey/hotkey.go] ── evdev key-down detected ──▶ program.Send(recordingStartedMsg)
    │
    ▼
[tui/update.go] ── starts recorder, plays start chime
    │
    ▼
[recorder/recorder.go] ── PortAudio captures PCM samples into []int16 buffer
    │
    │  (user releases F12)
    ▼
[hotkey/hotkey.go] ── evdev key-up detected ──▶ signals recorder to stop
    │
    ▼
[recorder/recorder.go] ── encodes PCM to WAV in memory ([]byte)
                        ──▶ program.Send(recordingStoppedMsg{wavData})
    │
    ▼
[tui/update.go] ── plays stop chime, dispatches tea.Cmd calling Transcriber.Transcribe()
    │
    ▼
[transcriber/openai.go] ── POST multipart/form-data to /v1/audio/transcriptions
                         ── reads response body as plain text
    │
    ▼
[tui/update.go] ── receives transcriptionResultMsg{text}
                 ── calls clipboard.PasteText(text)
    │
    ▼
[clipboard/clipboard.go] ── atotto/clipboard.WriteAll(text)
                          ── exec xdotool key ctrl+v
    │
    ▼
[Active application receives pasted text]
```

### 7.6 Concurrency Model

- **Main goroutine**: runs `bubbletea.Program`.
- **Hotkey goroutine**: blocks on evdev `Read()`, sends messages to the Bubble Tea program via `program.Send()`. Started once at boot, stopped on quit.
- **Recorder**: started/stopped on demand by the Bubble Tea update function. Runs PortAudio in a callback or blocking-read goroutine. When stopped, encodes WAV and sends `recordingStoppedMsg`.
- **Transcription**: executed as a `tea.Cmd` (which Bubble Tea runs in its own goroutine). Returns a `tea.Msg` on completion.
- **Chime playback**: fire-and-forget goroutine per chime.

### 7.7 Permissions Note

Reading `/dev/input/event*` requires either root access or membership in the `input` group:

```bash
sudo usermod -aG input $USER
# Then log out and log back in
```

---

## 8. Success Metrics

| # | Metric | Target |
|---|--------|--------|
| SM1 | End-to-end latency (release hotkey → text pasted) with a local transcription server | Under 3 seconds for a 5-second utterance. |
| SM2 | Binary size | Under 15 MB (statically linked, embedded chimes). |
| SM3 | Memory usage during recording | Under 50 MB RSS. |
| SM4 | Successful transcription rate on clear speech (English, quiet room) | Pipeline should not introduce failures beyond the model's own error rate. |
| SM5 | Zero-config startup | `palaver` with no arguments and no config file must work if a transcription server is running on `localhost:5092`. |

---

## 9. Open Questions

| # | Question | Impact | Suggested Resolution |
|---|----------|--------|---------------------|
| OQ1 | **Ollama audio support timeline**: Will Ollama ship `/v1/audio/transcriptions` before Palaver v1? | If yes, Ollama becomes a recommended provider. If no, the OpenAI-compatible provider pointing at Parakeet/whisper server is the default. | Design the interface now; swap the default later. Already accounted for. |
| OQ2 | **Wayland paste**: `xdotool` does not work on pure Wayland (no XWayland). Should v1 support `wtype` or `ydotool` as alternatives? | Users on Wayland-only sessions cannot paste. | Defer to v2. Document the X11/XWayland requirement. |
| OQ3 | **Multi-keyboard handling**: If the user has multiple input devices, which one should the evdev listener monitor? | Hotkey might not be detected if the wrong device is chosen. | Auto-detect by scanning `/dev/input/event*` for devices with `EV_KEY` capability. Allow user override in config. |
| OQ4 | **Audio device selection**: Should the user be able to pick a specific microphone? | Users with multiple mics may want to choose. | v1 uses the PortAudio default device. Add device selection in v2. |
| OQ5 | **Clipboard clobbering**: Pasting overwrites the clipboard. Should Palaver save/restore previous contents? | Minor UX annoyance. | Defer to v2. Document the behavior. |
| OQ6 | **Default chime files**: Where do the default WAV chimes come from? | Need to ship two small WAV files. | Generate two simple sine-wave tones programmatically during build, or include CC0-licensed chime files. |

---

## References & Sources

- [Vocalinux](https://github.com/jatinkrmalik/vocalinux) — similar Python/GTK voice-to-text app for Linux
- [VoiceInk](https://github.com/Beingpax/VoiceInk) — macOS voice-to-text app using whisper.cpp and Parakeet TDT
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — Go TUI framework (Elm architecture)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Go terminal styling library
- [achetronic/parakeet](https://github.com/achetronic/parakeet) — OpenAI-compatible ASR server using NVIDIA Parakeet TDT 0.6B (ONNX, CPU-only)
- [NVIDIA Parakeet TDT 0.6B v3](https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3) — 600M param multilingual ASR model
- [FluidAudio](https://github.com/FluidInference/FluidAudio) — Swift SDK for local audio AI (used by VoiceInk for Parakeet)
- [faster-whisper](https://github.com/SYSTRAN/faster-whisper) — CTranslate2-optimized Whisper inference
- [Speaches (faster-whisper-server)](https://github.com/speaches-ai/speaches) — OpenAI-compatible faster-whisper server
- [whisper.cpp](https://github.com/ggml-org/whisper.cpp) — C++ port of OpenAI Whisper
- [Ollama audio tracking issue #11798](https://github.com/ollama/ollama/issues/11798)
