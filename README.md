# Palaver

[![CI](https://github.com/Danondso/palaver/actions/workflows/ci.yml/badge.svg)](https://github.com/Danondso/palaver/actions/workflows/ci.yml)
[![Security](https://github.com/Danondso/palaver/actions/workflows/security.yml/badge.svg)](https://github.com/Danondso/palaver/actions/workflows/security.yml)

A voice-to-text transcription tool for Linux and macOS. Hold a hotkey, speak, release — your words are transcribed and pasted into the active application.

Built in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI and [Lip Gloss](https://github.com/charmbracelet/lipgloss) for styling. Ships with 4 themes: Synthwave (default), Everforest, Gruvbox, and Monochrome — press `t` to cycle or set in config. You can also define custom themes in `config.toml`.

## How It Works

1. Hold the hotkey (default: Right Ctrl on Linux, Cmd+Option on macOS)
2. Speak into your microphone
3. Release the hotkey
4. Audio is sent to a local transcription server
5. Optionally, transcribed text is rewritten by a local LLM (tone post-processing)
6. Text is pasted into your active application

All processing happens locally by default.

## Prerequisites

### System Dependencies

#### macOS

```bash
brew install portaudio whisper-cpp
```

#### Linux

```bash
# Ubuntu/Debian - core
sudo apt install libportaudio2 portaudio19-dev

# Paste support (pick one based on your display server)
# X11:
sudo apt install xdotool
# Wayland:
sudo apt install wl-clipboard ydotool
```

> **Wayland note:** Palaver uses `ydotool` which works via `/dev/uinput` at the
> kernel level, making it compatible with all Wayland compositors (GNOME, Sway,
> Cosmic, etc.). Palaver will auto-start `ydotoold` if it is not already running.
> Your user must have write access to `/dev/uinput` (typically via the `input` group).

### Permissions

#### macOS

Palaver needs two macOS permissions (grant in System Settings > Privacy & Security):

1. **Accessibility** — required for pasting text via simulated keystrokes. Add your terminal app (Terminal, iTerm2, etc.) to the list.
2. **Input Monitoring** — required for global hotkey detection via CGEventTap.

Microphone access is granted automatically on first run.

#### Linux

Palaver uses the Linux evdev subsystem for global hotkey detection. Your user must be in the `input` group:

```bash
sudo usermod -aG input $USER
# Log out and back in for the change to take effect
```

### Transcription Backend

Palaver needs a running transcription server that implements the OpenAI-compatible `POST /v1/audio/transcriptions` endpoint.

#### Option A: Managed Server (Recommended)

Palaver can automatically download and manage a local transcription server:

- **macOS:** Uses [whisper.cpp](https://github.com/ggml-org/whisper.cpp) (`whisper-server` from Homebrew) with the `ggml-base.en.bin` model (~150MB).
- **Linux:** Uses [Parakeet ASR Server](https://github.com/achetronic/parakeet) — NVIDIA Parakeet TDT 0.6B via ONNX, CPU-only, 3.97% WER (~670MB model files + ONNX Runtime).

```bash
palaver setup    # downloads server (Linux only) and model files
palaver          # auto-starts the managed server
```

The managed server stores files in `~/.local/share/palaver/` and auto-starts on launch when `server.auto_start = true` (the default). See the `[server]` config section below.

#### Option B: Manual Parakeet (Linux)

If you prefer to manage the server yourself:

```bash
curl -L -o parakeet https://github.com/achetronic/parakeet/releases/latest/download/parakeet-linux-amd64
chmod +x parakeet
make models    # downloads ~670MB model
./parakeet     # starts on port 5092
```

Or with Docker:
```bash
docker run -d -p 5092:5092 -v $(pwd)/models:/models ghcr.io/achetronic/parakeet:latest
```

Palaver defaults to `http://localhost:5092` — works out of the box.

#### Option C: faster-whisper-server

```bash
docker run -p 8000:8000 ghcr.io/speaches-ai/speaches:latest
```

Update config: `base_url = "http://localhost:8000"`

#### Option D: whisper.cpp server

```bash
git clone https://github.com/ggml-org/whisper.cpp.git && cd whisper.cpp
make -j && ./models/download-ggml-model.sh base.en
./build/bin/server -m models/ggml-base.en.bin --port 8080
```

Update config: `base_url = "http://localhost:8080"`

## Install

Download the latest release for your platform from [GitHub Releases](https://github.com/Danondso/palaver/releases), extract it, and run the install script:

### macOS (Apple Silicon)

```bash
# Download and extract the latest release
TAG=$(curl -sL https://api.github.com/repos/Danondso/palaver/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
curl -LO "https://github.com/Danondso/palaver/releases/download/${TAG}/palaver_${TAG}_darwin_arm64.tar.gz"
tar xzf "palaver_${TAG}_darwin_arm64.tar.gz"

# Install (installs dependencies, binary, and model files)
./install.sh
```

### Linux (amd64)

```bash
# Download and extract the latest release
TAG=$(curl -sL https://api.github.com/repos/Danondso/palaver/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
curl -LO "https://github.com/Danondso/palaver/releases/download/${TAG}/palaver_${TAG}_linux_amd64.tar.gz"
tar xzf "palaver_${TAG}_linux_amd64.tar.gz"

# Install (installs dependencies, binary, and model files)
./install.sh
```

The install script will:
- Install runtime dependencies using `sudo` (portaudio on Linux, portaudio + whisper-cpp on macOS, plus xdotool/ydotool for paste support)
- Copy the pre-built `palaver` binary to `~/.local/bin/`
- Run `palaver setup` to download the transcription model
- On Linux, add your user to the `input` group for hotkey access (a logout/login may be required for the group change to take effect)

## Build from Source

```bash
go build -o palaver ./cmd/palaver/
```

Or use the `install-from-source.sh` script, which installs build dependencies, compiles palaver, and runs setup in one step:

```bash
./install-from-source.sh
```

## Usage

```bash
./palaver           # normal mode
./palaver --debug   # verbose logging to stderr (hotkey events, WAV size, transcription timing, paste status)
./palaver setup     # download managed Parakeet server, ONNX Runtime, and models
```

The TUI displays the current state (idle/recording/transcribing/rewriting/pasting/error), the last transcription, and hotkey info. Press `q` or `Ctrl+C` to quit, `t` to cycle themes, `p` to cycle tone presets, `m` to cycle LLM models, `r` to restart the managed server.

## Uninstall

```bash
./uninstall.sh
```

Removes the binary, managed server data, and optionally the config directory. System packages (portaudio, whisper-cpp, xdotool, etc.) are not removed.

## Configuration

Config is loaded from `~/.config/palaver/config.toml`. If the file doesn't exist, defaults are used. Copy the example below and uncomment the settings you want to change:

```toml
# ~/.config/palaver/config.toml

# Theme: synthwave (default), everforest, gruvbox, or monochrome
# theme = "synthwave"

[hotkey]
# Linux: evdev key name (KEY_RIGHTCTRL, KEY_F12, KEY_SPACE, etc.)
# macOS: modifier combo (Cmd+Option, Option+Space, Ctrl+F5, etc.)
# key = "KEY_RIGHTCTRL"    # default: KEY_RIGHTCTRL (Linux), Cmd+Option (macOS)
# device = ""              # Linux only: empty = auto-detect keyboard

[audio]
# target_sample_rate = 16000  # resample to this rate for the transcription backend
# max_duration_sec = 60       # auto-stop recording after this many seconds
# chime_enabled = true        # set to false to disable chimes
# chime_start = ""            # path to custom start chime WAV (empty = built-in)
# chime_stop = ""             # path to custom stop chime WAV (empty = built-in)

[transcription]
# provider = "openai"                    # "openai" or "command"
# base_url = "http://localhost:5092"     # transcription server URL
# model = "whisper-1"                    # model name sent to the server
# timeout_sec = 30                       # transcription request timeout
# command = ""                           # for "command" provider: e.g. "whisper-cpp -f {input}"
# tls_skip_verify = false                # skip TLS cert verification (for self-signed certs)

[paste]
# mode = "type"         # default: "type" (Linux), "clipboard" (macOS)
#                       # "type" = direct typing (xdotool/ydotool on Linux, osascript keystroke on macOS)
#                       # "clipboard" = clipboard + paste shortcut (Cmd+V on macOS only)
#                       # NOTE: "clipboard" mode does not work on Linux — use "type" instead
# delay_ms = 50         # delay before paste (ms)

[server]
# auto_start = true     # auto-start managed server on launch
# data_dir = ""         # empty = ~/.local/share/palaver
# port = 5092           # port for managed server

[post_processing]
# enabled = false                          # enable LLM tone rewriting of transcriptions
# tone = "off"                             # off, formal, direct, token-efficient
# model = "llama3.2"                       # LLM model name (from Ollama or compatible API)
# base_url = "http://localhost:11434/v1"   # OpenAI-compatible chat completions endpoint
# timeout_sec = 10                         # post-processing request timeout
```

### Custom Themes

Define custom themes with `[[custom_theme]]` blocks and set `theme` to use one. Multiple custom themes can be defined — they are appended to the `t` key cycle after the built-in themes.

```toml
theme = "bedfellow"

[[custom_theme]]
name = "bedfellow"
primary = "#008585"
secondary = "#74A892"
accent = "#C7522A"
error = "#C7522A"
success = "#74A892"
warning = "#D97706"
background = "#1A1611"
text = "#FEF9E0"
dimmed = "#535A63"
separator = "#625647"
```

### Post-Processing (Tone Rewriting)

Palaver can optionally rewrite transcribed text using a local LLM before pasting. This is useful for cleaning up filler words, adjusting tone for emails, or making speech more concise. Post-processing uses an OpenAI-compatible chat completions API (defaults to [Ollama](https://ollama.com/) on `localhost:11434`).

Built-in tones: `formal`, `direct`, `token-efficient`. Press `p` at runtime to cycle through tones, or `m` to cycle available models. When tone is set to `off`, post-processing is bypassed entirely.

If the LLM is unavailable or returns an error, Palaver gracefully falls back to pasting the original transcription.

```toml
[post_processing]
enabled = true
tone = "formal"
model = "llama3.2"
base_url = "http://localhost:11434/v1"
timeout_sec = 10
```

### Custom Tones

Define custom tone presets with `[[custom_tone]]` blocks. Custom tones are appended to the `p` key cycle. You can also override built-in tones by using the same name.

```toml
[post_processing]
enabled = true
tone = "pirate"

[[custom_tone]]
name = "pirate"
prompt = "Rewrite the following transcribed speech as a pirate would say it. Keep the meaning identical. Return only the rewritten text, no explanation."
```

### Custom Chimes

Provide your own WAV files:

```toml
[audio]
chime_start = "/path/to/my-start-chime.wav"
chime_stop = "/path/to/my-stop-chime.wav"
```

Or disable chimes entirely:

```toml
[audio]
chime_enabled = false
```

### Command Provider

For backends without an HTTP API, use the command provider:

```toml
[transcription]
provider = "command"
command = "whisper-cpp --model base.en --file {input}"
```

`{input}` is replaced with the path to a temporary WAV file.

## Architecture

```
cmd/palaver/main.go                  Entry point, wiring
cmd/palaver/entry_{linux,darwin}.go   Platform-specific entry
cmd/palaver/hotkey_{linux,darwin}.go  Platform-specific hotkey wiring
internal/config/                      TOML config loading (platform-specific defaults)
internal/hotkey/                      Global hotkey: evdev (Linux), CGEventTap (macOS)
internal/recorder/                    PortAudio capture, resampling, WAV encoding
internal/transcriber/                 Transcriber interface + OpenAI/Command providers
internal/clipboard/                   Paste: xdotool/ydotool (Linux), pbcopy/osascript (macOS)
internal/chime/                       Audio chime playback via beep
internal/postprocess/                 LLM tone rewriting via chat completions API
internal/server/                      Managed server: Parakeet (Linux), whisper-cpp (macOS)
internal/tui/                         Bubble Tea model + Lip Gloss view
```

## Audio Resampling

Palaver captures audio at your device's native sample rate (typically 44.1kHz or 48kHz) and resamples to 16kHz using polyphase FIR filtering with Kaiser window design via [go-audio-resampling](https://github.com/tphakala/go-audio-resampling). This provides professional-grade quality suitable for speech recognition without any external C library dependencies.

## License

MIT
