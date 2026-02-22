//go:build darwin

package server

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func libExtension() string {
	return ".dylib"
}

func libraryPathEnvVar() string {
	return "DYLD_LIBRARY_PATH"
}

func parakeetAvailable() bool {
	return true
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

// verifyBinary checks that a file starts with valid Mach-O magic bytes.
func verifyBinary(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("read magic bytes: %w", err)
	}

	// Check for Mach-O magic bytes:
	// 0xFEEDFACF = 64-bit Mach-O (little-endian, ARM64 or x86_64)
	// 0xFEEDFACE = 32-bit Mach-O (little-endian)
	// 0xCAFEBABE = Universal binary (fat binary)
	// 0xBEBAFECA = Universal binary (little-endian representation)
	if (magic[0] == 0xCF && magic[1] == 0xFA && magic[2] == 0xED && magic[3] == 0xFE) || // 64-bit LE
		(magic[0] == 0xCE && magic[1] == 0xFA && magic[2] == 0xED && magic[3] == 0xFE) || // 32-bit LE
		(magic[0] == 0xCA && magic[1] == 0xFE && magic[2] == 0xBA && magic[3] == 0xBE) || // Universal BE
		(magic[0] == 0xBE && magic[1] == 0xBA && magic[2] == 0xFE && magic[3] == 0xCA) { // Universal LE
		return nil
	}

	return fmt.Errorf("not a valid Mach-O binary (got %x)", magic)
}

// systemOnnxRuntimeAvailable returns false on macOS (no ldconfig).
func systemOnnxRuntimeAvailable() bool {
	return false
}
