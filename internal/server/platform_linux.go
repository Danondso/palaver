//go:build linux

package server

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func libExtension() string {
	return ".so"
}

func serverBinaryName() string {
	return "parakeet"
}

func resolveServerBinary(dataDir string) string {
	return filepath.Join(dataDir, "parakeet")
}

func isServerInstalled(binaryPath, modelsDir string) bool {
	if _, err := os.Stat(binaryPath); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(modelsDir, "encoder-model.int8.onnx")); err != nil {
		return false
	}
	return true
}

func serverArgs(port int, modelsDir string) []string {
	return []string{
		"-port", fmt.Sprintf("%d", port),
		"-models", modelsDir,
	}
}

func serverEnv(onnxDir string) []string {
	onnxLib := filepath.Join(onnxDir, "libonnxruntime.so")
	return []string{
		fmt.Sprintf("ONNXRUNTIME_LIB=%s", onnxLib),
		fmt.Sprintf("LD_LIBRARY_PATH=%s:%s", onnxDir, os.Getenv("LD_LIBRARY_PATH")),
	}
}

func healthCheckURL(port int) string {
	return fmt.Sprintf("http://localhost:%d/v1/models", port)
}

func healthCheckTimeout() time.Duration {
	return 120 * time.Second
}

func serverReadyLog() string {
	return "parakeet is ready"
}

func needsOnnxRuntime() bool {
	return true
}

func setupServer(binaryPath, modelsDir, onnxDir string, logger *log.Logger, progress ProgressFunc) error {
	// Download binary
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		logger.Printf("downloading parakeet binary...")
		url := parakeetBinaryURL()
		checksum, err := downloadFile(url, binaryPath, progress, "binary")
		if err != nil {
			return fmt.Errorf("download parakeet binary: %w", err)
		}
		logger.Printf("binary SHA256: %s", checksum)
		if err := verifyBinary(binaryPath); err != nil {
			os.Remove(binaryPath)
			return fmt.Errorf("downloaded binary is invalid: %w", err)
		}
		if err := os.Chmod(binaryPath, 0o755); err != nil { //nolint:gosec // binary must be executable
			return fmt.Errorf("chmod parakeet binary: %w", err)
		}
	}

	// Download models
	models := parakeetModelURLs()
	for filename, url := range models {
		dest := filepath.Join(modelsDir, filename)
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			logger.Printf("downloading model file: %s", filename)
			if _, err := downloadFile(url, dest, progress, filename); err != nil {
				return fmt.Errorf("download model %s: %w", filename, err)
			}
		}
	}

	// Download ONNX Runtime if not available
	matches, _ := filepath.Glob(filepath.Join(onnxDir, "libonnxruntime.so*"))
	if len(matches) == 0 && !systemOnnxRuntimeAvailable() {
		logger.Printf("downloading ONNX Runtime %s...", onnxRuntimeVersion)
		if err := downloadAndExtractOnnxRuntime(onnxDir, progress); err != nil {
			return fmt.Errorf("download onnxruntime: %w", err)
		}
	}

	return nil
}

// parakeetBinaryURL returns the GitHub release URL for the parakeet binary.
func parakeetBinaryURL() string {
	arch := runtime.GOARCH
	return fmt.Sprintf(
		"https://github.com/achetronic/parakeet/releases/latest/download/parakeet-linux-%s",
		arch,
	)
}

// parakeetModelURLs returns a map of filename → HuggingFace download URL for the
// INT8-quantized Parakeet TDT 0.6B v2 ONNX model files.
func parakeetModelURLs() map[string]string {
	base := "https://huggingface.co/istupakov/parakeet-tdt-0.6b-v2-onnx/resolve/main"
	return map[string]string{
		"config.json":                   base + "/config.json",
		"vocab.txt":                     base + "/vocab.txt",
		"encoder-model.int8.onnx":       base + "/encoder-model.int8.onnx",
		"decoder_joint-model.int8.onnx": base + "/decoder_joint-model.int8.onnx",
	}
}

// verifyBinary checks that a file starts with the ELF magic bytes, providing
// a basic integrity check that the downloaded binary is a valid executable.
func verifyBinary(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("read magic bytes: %w", err)
	}
	// ELF magic: 0x7f 'E' 'L' 'F'
	if magic[0] != 0x7f || magic[1] != 'E' || magic[2] != 'L' || magic[3] != 'F' {
		return fmt.Errorf("not a valid ELF binary (got %x)", magic)
	}
	return nil
}

// systemOnnxRuntimeAvailable checks if ONNX Runtime is available system-wide via ldconfig.
func systemOnnxRuntimeAvailable() bool {
	out, err := exec.Command("ldconfig", "-p").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "libonnxruntime.so") {
				return true
			}
		}
	}
	return false
}
