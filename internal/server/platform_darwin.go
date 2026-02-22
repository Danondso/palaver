//go:build darwin

package server

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func libExtension() string {
	return ".dylib"
}

func serverBinaryName() string {
	return "whisper-server"
}

func serverBinaryPath() string {
	path, err := exec.LookPath("whisper-server")
	if err != nil {
		return ""
	}
	return path
}

func resolveServerBinary(dataDir string) string {
	if p := serverBinaryPath(); p != "" {
		return p
	}
	return "whisper-server"
}

func isServerInstalled(binaryPath, modelsDir string) bool {
	if _, err := exec.LookPath(binaryPath); err != nil {
		if _, err := os.Stat(binaryPath); err != nil {
			return false
		}
	}
	if _, err := os.Stat(filepath.Join(modelsDir, "ggml-base.en.bin")); err != nil {
		return false
	}
	return true
}

func serverArgs(port int, modelsDir string) []string {
	return []string{
		"--model", filepath.Join(modelsDir, "ggml-base.en.bin"),
		"--port", fmt.Sprintf("%d", port),
		"--host", "127.0.0.1",
		"--inference-path", "/v1/audio/transcriptions",
		"--language", "en",
		"--no-timestamps",
	}
}

func serverEnv(onnxDir string) []string {
	return nil
}

func healthCheckURL(port int) string {
	return fmt.Sprintf("http://localhost:%d/", port)
}

func healthCheckTimeout() time.Duration {
	return 30 * time.Second
}

func serverReadyLog() string {
	return "whisper-server is ready"
}

func needsOnnxRuntime() bool {
	return false
}

func setupServer(binaryPath, modelsDir, onnxDir string, logger *log.Logger, progress ProgressFunc) error {
	// Check that whisper-server is in PATH
	if _, err := exec.LookPath("whisper-server"); err != nil {
		logger.Printf("whisper-server not found in PATH")
		logger.Printf("Install it with: brew install whisper-cpp")
		return fmt.Errorf("whisper-server not found: install with 'brew install whisper-cpp'")
	}

	// Download ggml-base.en.bin model if missing
	modelPath := filepath.Join(modelsDir, "ggml-base.en.bin")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		logger.Printf("downloading whisper model: ggml-base.en.bin")
		url := "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin"
		checksum, err := downloadFile(url, modelPath, progress, "ggml-base.en.bin")
		if err != nil {
			return fmt.Errorf("download whisper model: %w", err)
		}
		logger.Printf("model SHA256: %s", checksum)
	}

	return nil
}

// systemOnnxRuntimeAvailable returns false on macOS (no ldconfig).
func systemOnnxRuntimeAvailable() bool {
	return false
}
