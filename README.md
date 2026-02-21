# Palaver

A voice-to-text transcription tool for Linux. Hold a hotkey, speak, release — your words are transcribed and pasted into the active application.

Built in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI and [Lip Gloss](https://github.com/charmbracelet/lipgloss) for 80s Miami synthwave styling.

## How It Works

1. Hold the hotkey (default: Right Ctrl)
2. Speak into your microphone
3. Release the hotkey
4. Audio is sent to a local transcription server
5. Transcribed text is pasted into your active application

All processing happens locally by default.

## Prerequisites

### System Dependencies

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

### Input Device Permissions

Palaver uses the Linux evdev subsystem for global hotkey detection. Your user must be in the `input` group:

```bash
sudo usermod -aG input $USER
# Log out and back in for the change to take effect
```

### Transcription Backend

Palaver needs a running transcription server that implements the OpenAI-compatible `POST /v1/audio/transcriptions` endpoint.

#### Option A: NVIDIA Parakeet (Recommended)

[Parakeet ASR Server](https://github.com/achetronic/parakeet) — NVIDIA Parakeet TDT 0.6B via ONNX, CPU-only, 3.97% WER.

```bash
# Download and run
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

#### Option B: faster-whisper-server

```bash
docker run -p 8000:8000 ghcr.io/speaches-ai/speaches:latest
```

Update config: `base_url = "http://localhost:8000"`

#### Option C: whisper.cpp server

```bash
git clone https://github.com/ggml-org/whisper.cpp.git && cd whisper.cpp
make -j && ./models/download-ggml-model.sh base.en
./build/bin/server -m models/ggml-base.en.bin --port 8080
```

Update config: `base_url = "http://localhost:8080"`

## Build

```bash
go build -o palaver ./cmd/palaver/
```

## Usage

```bash
./palaver           # normal mode
./palaver --debug   # verbose logging to stderr (hotkey events, WAV size, transcription timing, paste status)
```

The TUI displays the current state (idle/recording/transcribing/error), the last transcription, and hotkey info. Press `q` or `Ctrl+C` to quit.

## Configuration

Config is loaded from `~/.config/palaver/config.toml`. If the file doesn't exist, defaults are used.

```toml
[hotkey]
key = "KEY_RIGHTCTRL"    # evdev key name (KEY_F12, KEY_SPACE, etc.)
device = ""              # empty = auto-detect keyboard

[audio]
target_sample_rate = 16000  # resample to this rate for the transcription backend
max_duration_sec = 60       # auto-stop recording after this many seconds
chime_start = ""            # path to custom start chime WAV (empty = built-in)
chime_stop = ""             # path to custom stop chime WAV (empty = built-in)
chime_enabled = true        # set to false to disable chimes

[transcription]
provider = "openai"                    # "openai" or "command"
base_url = "http://localhost:5092"     # transcription server URL
model = "default"                      # model name sent to the server
timeout_sec = 30                       # transcription request timeout
command = ""                           # for "command" provider: e.g. "whisper-cpp -f {input}"
tls_skip_verify = false                # skip TLS certificate verification (for self-signed certs)

[paste]
delay_ms = 50     # delay before paste (ms)
mode = "type"     # "type" (direct typing, works in terminals) or "clipboard" (Ctrl+V)
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
cmd/palaver/main.go          Entry point, wiring
internal/config/              TOML config loading
internal/hotkey/              evdev global hotkey listener
internal/recorder/            PortAudio capture, resampling, WAV encoding
internal/transcriber/         Transcriber interface + OpenAI/Command providers
internal/clipboard/           Paste: atotto/clipboard+xdotool (X11), wl-copy+ydotool (Wayland)
internal/chime/               Audio chime playback via beep
internal/tui/                 Bubble Tea model + Lip Gloss view
```

## Audio Resampling

Palaver captures audio at your device's native sample rate (typically 44.1kHz or 48kHz) and resamples to 16kHz using polyphase FIR filtering with Kaiser window design via [go-audio-resampling](https://github.com/tphakala/go-audio-resampling). This provides professional-grade quality suitable for speech recognition without any external C library dependencies.

## License

MIT
