# Palaver — macOS Compatibility PRD

## 1. Introduction / Overview

This PRD covers adding macOS support to Palaver while retaining full Linux compatibility. The goal is a single codebase that compiles and runs natively on both platforms using Go build tags (`//go:build linux` / `//go:build darwin`) to separate platform-specific implementations behind shared interfaces.

Palaver currently depends on several Linux-only subsystems: evdev for global hotkeys, xdotool/ydotool for text pasting, `pactl` for microphone detection, and a Linux-specific managed server setup. Each must be replaced with a macOS equivalent on Darwin builds.

### Reference: VoiceInk (macOS voice-to-text app)

[VoiceInk](https://github.com/Beingpax/VoiceInk) is a mature macOS voice-to-text app that Palaver is inspired by. Its architecture informs several design decisions in this PRD:

- **Hotkeys:** Uses `NSEvent.addGlobalMonitorForEvents(matching: .flagsChanged)` for modifier keys + [sindresorhus/KeyboardShortcuts](https://github.com/sindresorhus/KeyboardShortcuts) (wrapping Carbon `RegisterEventHotKey`) for custom combos. Supports a dual-mode UX: hold >0.5s = push-to-talk, quick tap = hands-free toggle.
- **Text pasting:** Uses `NSPasteboard` + `CGEvent` to simulate Cmd+V. Temporarily switches keyboard input source to QWERTY before posting virtual key events so key codes map correctly regardless of user's layout. Marks clipboard entries as "transient" so clipboard managers (Paste, Maccy) don't save them. Optionally saves/restores previous clipboard contents.
- **Audio:** Uses low-level Core Audio AUHAL (`AudioUnit` HAL Output), not PortAudio or AVAudioEngine. Captures at device native rate, resamples to 16kHz mono 16-bit PCM via linear interpolation.
- **Transcription:** Supports 4 backends — whisper.cpp (C bindings with Metal GPU), FluidAudio/Parakeet (local), OpenAI-compatible cloud API, and Apple SpeechAnalyzer (macOS 26+). Also supports real-time streaming via WebSocket (Deepgram, ElevenLabs, etc.).
- **Permissions:** Runs with app sandbox **disabled**. Checks `AXIsProcessTrusted()` before posting CGEvents. Microphone permission is prompted automatically by macOS on first Core Audio access.

---

## 2. Goals

| # | Goal |
|---|------|
| G1 | Palaver MUST compile and run natively on macOS (ARM64 Apple Silicon). Intel Mac support is a stretch goal. |
| G2 | All existing Linux functionality MUST continue to work without regression. |
| G3 | Platform-specific code MUST be isolated behind interfaces and build tags — no runtime `if runtime.GOOS == "darwin"` in shared code paths. |
| G4 | The core data flow (hotkey → record → transcribe → paste) MUST work identically on both platforms. |
| G5 | macOS users MUST be able to use any OpenAI-compatible transcription backend (cloud API, whisper.cpp, Speaches, etc.). |

---

## 3. User Stories

| ID | As a... | I want to... | So that... |
|----|---------|-------------|------------|
| US1 | macOS user | Install Palaver via `go install` or `go build` | I can run it on my Mac without manual patching. |
| US2 | macOS user | Hold a hotkey, speak, and release to have text pasted into my active app | I get the same voice-to-text workflow as Linux users. |
| US3 | macOS user | Grant accessibility and microphone permissions once | Palaver can capture hotkeys and simulate keystrokes. |
| US4 | macOS user | Use `palaver setup` to install a local transcription server | I can run transcription locally on my Mac. |
| US5 | Linux user | Continue using Palaver exactly as before | macOS support does not break or degrade my workflow. |

---

## 4. Functional Requirements

### 4.1 Global Hotkey (macOS)

VoiceInk uses `NSEvent` global monitors for modifier keys and Carbon `RegisterEventHotKey` (via KeyboardShortcuts library) for custom combos. It also implements a dual-mode UX: hold >0.5s = push-to-talk, quick tap (<0.5s) = hands-free toggle. Palaver v1 on macOS will implement hold-to-record only, matching the existing Linux behavior.

| # | Requirement |
|---|-------------|
| FR1 | On macOS, global hotkey capture MUST use the [`golang.design/x/hotkey`](https://github.com/golang-design/hotkey) library, which wraps macOS Carbon/Cocoa APIs. |
| FR2 | The library provides separate `Keydown()` and `Keyup()` channels, matching the existing push-to-talk (hold-to-record) behavior. |
| FR3 | The hotkey listener MUST run on the main thread on macOS (required by the OS). Use `golang.design/x/hotkey/mainthread` for initialization. |
| FR4 | The macOS default hotkey MUST be `Right Option` (matching VoiceInk convention). The config MUST accept key names that map to `golang.design/x/hotkey` key codes. A mapping from evdev key names to macOS key codes MUST be provided so that existing configs work cross-platform where possible. |
| FR5 | macOS requires the user to grant **Accessibility** permissions to the terminal app running Palaver. The TUI SHOULD display a helpful message if hotkey registration fails due to missing permissions. |

**Interface:**

```go
// internal/hotkey/listener.go (shared)
type Listener interface {
    Start(ctx context.Context, onDown func(), onUp func()) error
    Stop() error
    KeyName() string
}
```

**Files:**
- `internal/hotkey/hotkey.go` → rename to `internal/hotkey/hotkey_linux.go` (add `//go:build linux`)
- New: `internal/hotkey/hotkey_darwin.go` (`//go:build darwin`)
- New: `internal/hotkey/listener.go` (shared interface)

### 4.2 Clipboard & Text Pasting (macOS)

VoiceInk uses `NSPasteboard` + `CGEvent` Cmd+V for pasting, which is more reliable than AppleScript. It also temporarily switches the keyboard input source to QWERTY so that virtual key code `0x09` always maps to "V" regardless of the user's keyboard layout. Palaver should follow this proven approach.

| # | Requirement |
|---|-------------|
| FR6 | On macOS, clipboard write MUST use `pbcopy` (native CLI tool, no dependencies). |
| FR7 | In "clipboard" paste mode (the default and recommended mode on macOS), text MUST be written to clipboard via `pbcopy`, then `Cmd+V` MUST be simulated. The keystroke simulation SHOULD use `osascript -e 'tell application "System Events" to keystroke "v" using command down'`. A future enhancement could use CGEvent via CGo for better reliability across keyboard layouts (as VoiceInk does). |
| FR8 | In "type" paste mode, text MUST be simulated via AppleScript: `osascript -e 'tell application "System Events" to keystroke "<text>"'`. Special characters MUST be escaped. Note: this mode may have issues with non-ASCII text and non-QWERTY layouts; clipboard mode is preferred on macOS. |
| FR9 | The paste module MUST auto-detect the platform and use the correct implementation. |
| FR10 | Accessibility permission (`AXIsProcessTrusted()`) is required for keystroke simulation. If not granted, the paste operation SHOULD log a clear error and the TUI SHOULD display guidance to enable it in System Settings > Privacy & Security > Accessibility. |

**Files:**
- `internal/clipboard/clipboard.go` → rename to `internal/clipboard/clipboard_linux.go` (add `//go:build linux`)
- New: `internal/clipboard/clipboard_darwin.go` (`//go:build darwin`)
- New: `internal/clipboard/clipboard.go` (shared interface/factory)

### 4.3 Audio Recording (macOS)

| # | Requirement |
|---|-------------|
| FR10 | PortAudio (`github.com/gordonklaus/portaudio`) works on macOS via CoreAudio — no change needed for core audio capture. |
| FR11 | The `pactl`-based microphone name detection (`micNameFromPactl()`) MUST be skipped on macOS. On macOS, use the PortAudio device name directly. |
| FR12 | macOS requires the user to grant **Microphone** permission. The application SHOULD handle the permission prompt gracefully (PortAudio will trigger the macOS permission dialog on first use). |

**Files:**
- Extract `micNameFromPactl()` from `internal/recorder/recorder.go` into `internal/recorder/mic_linux.go`
- New: `internal/recorder/mic_darwin.go` (returns PortAudio device name directly)

### 4.4 Managed Server & Setup (macOS)

| # | Requirement |
|---|-------------|
| FR13 | `achetronic/parakeet` does NOT publish macOS binaries (only `parakeet-linux-amd64` and `parakeet-linux-arm64` are available). The `palaver setup` command on macOS MUST inform the user that the managed Parakeet server is not available on macOS and suggest alternative backends. |
| FR14 | ONNX Runtime publishes `onnxruntime-osx-arm64` builds. If/when a macOS Parakeet binary becomes available, the download logic MUST be updated to fetch the correct ONNX Runtime for the platform. |
| FR15 | On macOS, the server binary verification MUST check for Mach-O magic bytes (`0xCFFAEDFE` for ARM64, `0xCEFAEDFE` for x86_64, `0xBEBAFECA` for universal) instead of ELF (`0x7F454C46`). |
| FR16 | On macOS, `DYLD_LIBRARY_PATH` MUST be used instead of `LD_LIBRARY_PATH` for ONNX Runtime discovery. |
| FR17 | The `ldconfig -p` check for system ONNX Runtime MUST be skipped on macOS (no ldconfig equivalent). |
| FR18 | On macOS, shared libraries use `.dylib` extension instead of `.so`. Download and extraction logic MUST account for this. |

**Files:**
- `internal/server/download.go` — add platform-aware URL construction and binary verification
- `internal/server/server.go` — add platform-aware library path env var

### 4.5 Configuration Paths (macOS)

| # | Requirement |
|---|-------------|
| FR19 | On macOS, the default config path MUST be `~/.config/palaver/config.toml` (same as Linux, following XDG convention which works on macOS). |
| FR20 | On macOS, the default data directory MUST be `~/.local/share/palaver` (same as Linux). This avoids platform-specific path logic and keeps configs portable. |

**Rationale:** While macOS convention is `~/Library/Application Support/`, Palaver is a terminal tool whose users expect Unix-style paths. Keeping paths identical simplifies the codebase and documentation.

### 4.6 Main Entry Point (macOS)

| # | Requirement |
|---|-------------|
| FR21 | The `syscall.Dup`/`syscall.Dup2` stderr suppression in `main.go` uses POSIX calls that work on both Linux and macOS. No change needed. |
| FR22 | On macOS, the `golang.design/x/hotkey/mainthread` package requires the main goroutine to be locked to the OS thread. The entry point MUST call `mainthread.Init()` on macOS to satisfy this requirement. |

### 4.7 Install & Uninstall Scripts (macOS)

| # | Requirement |
|---|-------------|
| FR23 | `install.sh` MUST detect macOS (`uname -s` = `Darwin`) and skip Linux-specific package installation (apt/dnf/pacman, `input` group, `usermod`). |
| FR24 | On macOS, `install.sh` MUST check for Homebrew and install `portaudio` via `brew install portaudio` if not present. |
| FR25 | On macOS, `install.sh` MUST skip or warn about `palaver setup` since the managed Parakeet server is unavailable on macOS. |
| FR26 | `uninstall.sh` MUST work on macOS by detecting the platform and adjusting paths and package names accordingly. |

---

## 5. Non-Goals (Out of Scope)

- **Native macOS menu bar app / SwiftUI wrapper** — Palaver remains a TUI on all platforms.
- **macOS Parakeet server** — upstream does not provide macOS builds; this is out of our control.
- **Intel Mac (x86_64) support** — stretch goal. Apple Silicon (ARM64) is the primary target.
- **Windows support** — not addressed in this PRD.
- **CoreML / MLX transcription** — users can run whisper.cpp or a cloud API; we don't bundle a macOS-native model runtime.
- **Automatic accessibility/microphone permission requests** — macOS handles these via system dialogs. Palaver provides guidance in error messages.

---

## 6. Technical Considerations

### 6.1 Build Tags Strategy

All platform-specific code uses Go build tags. No file contains both Linux and macOS implementations.

```
internal/
  hotkey/
    listener.go          # Shared Listener interface
    hotkey_linux.go      # //go:build linux  — evdev implementation
    hotkey_darwin.go     # //go:build darwin — golang.design/x/hotkey implementation
    hotkey_test.go       # Shared tests (mock-based)
  clipboard/
    clipboard.go         # Shared interface + factory
    clipboard_linux.go   # //go:build linux  — xdotool/ydotool/wl-copy
    clipboard_darwin.go  # //go:build darwin — pbcopy/osascript
  recorder/
    recorder.go          # Shared PortAudio recording (cross-platform)
    mic_linux.go         # //go:build linux  — pactl mic name detection
    mic_darwin.go        # //go:build darwin — PortAudio device name fallback
  server/
    server.go            # Shared server lifecycle (cross-platform)
    download.go          # Platform-aware download URLs and binary verification
```

### 6.2 New Dependencies

| Dependency | Purpose | Platform |
|------------|---------|----------|
| `golang.design/x/hotkey` | Global hotkey registration | macOS (darwin) |
| `golang.design/x/hotkey/mainthread` | Main thread event loop | macOS (darwin) |

### 6.3 macOS System Dependencies

| Dependency | Source | Purpose |
|------------|--------|---------|
| PortAudio | `brew install portaudio` | Audio capture via CoreAudio |
| Accessibility permission | System Preferences → Privacy & Security → Accessibility | Global hotkey capture + keystroke simulation |
| Microphone permission | System Preferences → Privacy & Security → Microphone | Audio recording |

### 6.4 Key Name Mapping

The config uses Linux evdev key names (e.g., `KEY_RIGHTCTRL`). On macOS, these must map to `golang.design/x/hotkey` key/modifier constants:

| evdev Name | macOS Mapping |
|------------|---------------|
| `KEY_RIGHTCTRL` | `hotkey.KeyRight` + `hotkey.ModCtrl` (or just `ModCtrl` if right-specific not available) |
| `KEY_F12` | `hotkey.KeyF12` |
| `KEY_F5` | `hotkey.KeyF5` |
| `KEY_RIGHTALT` | `hotkey.ModOption` |

A mapping table in `hotkey_darwin.go` handles the translation. Unsupported keys produce a clear error message.

### 6.5 Managed Server on macOS — User Guidance

Since `achetronic/parakeet` has no macOS builds, `palaver setup` on macOS MUST print:

```
The managed Parakeet server is not available on macOS.

Recommended alternatives:
  1. whisper.cpp server (local, CPU):
     brew install whisper-cpp
     whisper-server -m ggml-base.en.bin --port 5092

  2. Cloud API (Groq — fast, free tier):
     Set in ~/.config/palaver/config.toml:
       [transcription]
       base_url = "https://api.groq.com/openai"
       model = "whisper-large-v3"
     Export OPENAI_API_KEY with your Groq key.

  3. Speaches (Docker):
     docker run -p 5092:5092 ghcr.io/speaches-ai/speaches:latest
```

---

## 7. Implementation Order

| Phase | Scope | Packages Affected |
|-------|-------|-------------------|
| 1 | Define shared interfaces, split existing Linux code into `_linux.go` files with build tags | `hotkey`, `clipboard`, `recorder` |
| 2 | Implement macOS hotkey listener (`hotkey_darwin.go`) with `golang.design/x/hotkey` | `hotkey` |
| 3 | Implement macOS clipboard/paste (`clipboard_darwin.go`) with `pbcopy`/`osascript` | `clipboard` |
| 4 | Implement macOS mic detection (`mic_darwin.go`) — use PortAudio device name | `recorder` |
| 5 | Update managed server for platform awareness (URLs, binary format, env vars, macOS setup guidance) | `server` |
| 6 | Update `main.go` for macOS main thread requirement | `cmd/palaver` |
| 7 | Update `install.sh` and `uninstall.sh` for macOS | scripts |
| 8 | Cross-platform testing, README updates | docs |

---

## 8. Success Metrics

| # | Metric | Target |
|---|--------|--------|
| SM1 | `go build` succeeds on macOS ARM64 with only `brew install portaudio` as prerequisite | Pass |
| SM2 | All existing tests pass on Linux with no changes to test files | Pass |
| SM3 | Push-to-talk hotkey works on macOS with Accessibility permission granted | Pass |
| SM4 | Text is correctly pasted into the active macOS application after transcription | Pass |
| SM5 | `palaver setup` on macOS provides clear alternative backend guidance | Pass |
| SM6 | End-to-end latency on macOS matches Linux (under 3 seconds for 5-second utterance with local backend) | Pass |

---

## 9. Resolved Questions

| # | Question | Resolution |
|---|----------|------------|
| RQ1 | Will `achetronic/parakeet` ever ship macOS builds? | Monitor upstream. Guide users to alternatives for now. |
| RQ2 | Should the macOS default hotkey be different? | **Yes — default to Right Option on macOS** (matching VoiceInk convention). Right Ctrl remains the Linux default. |
| RQ3 | Does `golang.design/x/hotkey` support key-down vs key-up? | **Yes** — provides `Keydown()` and `Keyup()` channels. |
| RQ4 | Add VoiceInk-style dual-mode (quick tap = hands-free, long press = push-to-talk)? | **No** — not for this iteration. Hold-to-record only. |
| RQ5 | Should clipboard paste use CGEvent via CGo or AppleScript? | **AppleScript** — simpler, no CGo dependency. Revisit if layout issues arise. |
| RQ6 | AppleScript `keystroke` handling of special characters? | **Default to clipboard mode on macOS.** Type mode is a fallback. |
| RQ7 | Intel Mac support? | **Apple Silicon only** for now. |
| RQ8 | Transient clipboard entries like VoiceInk? | **Not needed** — Palaver already clears the clipboard after paste (100ms delay). The transient `NSPasteboard` flag would require CGo. Current clear-after-paste is sufficient. |

---

## References

- [VoiceInk](https://github.com/Beingpax/VoiceInk) — macOS voice-to-text app; primary reference for macOS patterns (hotkey, paste, permissions)
- [sindresorhus/KeyboardShortcuts](https://github.com/sindresorhus/KeyboardShortcuts) — Swift library used by VoiceInk for custom hotkey combos (wraps Carbon `RegisterEventHotKey`)
- [FluidAudio](https://github.com/FluidInference/FluidAudio) — Swift SDK used by VoiceInk for local Parakeet inference on macOS
- [`golang.design/x/hotkey`](https://github.com/golang-design/hotkey) — Cross-platform global hotkey library for Go (macOS, Linux, Windows)
- [`achetronic/parakeet` releases](https://github.com/achetronic/parakeet/releases) — Linux-only binaries (amd64, arm64)
- [ONNX Runtime releases](https://github.com/microsoft/onnxruntime/releases) — macOS ARM64 builds available (`onnxruntime-osx-arm64`)
- [PortAudio](http://www.portaudio.com/) — Cross-platform audio I/O (CoreAudio on macOS)
- [Apple Accessibility permissions](https://support.apple.com/guide/mac-help/allow-accessibility-apps-to-access-your-mac-mh43185/mac)
