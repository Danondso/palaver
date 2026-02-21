package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Danondso/palaver/internal/config"
)

// Server manages the lifecycle of a bundled Parakeet transcription server.
type Server struct {
	BinaryPath string
	ModelsDir  string
	OnnxDir    string // directory containing libonnxruntime.so
	Port       int
	Logger     *log.Logger

	cmd *exec.Cmd
	mu  sync.Mutex
}

// New creates a Server with paths resolved from the config.
func New(cfg *config.ServerConfig, logger *log.Logger) *Server {
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = config.DefaultDataDir()
	}
	return &Server{
		BinaryPath: filepath.Join(dataDir, "parakeet"),
		ModelsDir:  filepath.Join(dataDir, "models"),
		OnnxDir:    filepath.Join(dataDir, "onnxruntime"),
		Port:       cfg.Port,
		Logger:     logger,
	}
}

// IsInstalled returns true if the binary, encoder model, and ONNX Runtime exist.
func (s *Server) IsInstalled() bool {
	if _, err := os.Stat(s.BinaryPath); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(s.ModelsDir, "encoder-model.int8.onnx")); err != nil {
		return false
	}
	if !s.onnxRuntimeAvailable() {
		return false
	}
	return true
}

// onnxRuntimeAvailable checks if ONNX Runtime is available either system-wide
// or in our bundled OnnxDir.
func (s *Server) onnxRuntimeAvailable() bool {
	// Check bundled copy first
	matches, _ := filepath.Glob(filepath.Join(s.OnnxDir, "libonnxruntime.so*"))
	if len(matches) > 0 {
		return true
	}
	// Check if it's available system-wide via ldconfig
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

// Setup downloads the Parakeet binary and model files if they are missing.
func (s *Server) Setup(progress ProgressFunc) error {
	// Download binary
	if _, err := os.Stat(s.BinaryPath); os.IsNotExist(err) {
		s.Logger.Printf("downloading parakeet binary...")
		url := parakeetBinaryURL()
		checksum, err := downloadFile(url, s.BinaryPath, progress, "binary")
		if err != nil {
			return fmt.Errorf("download parakeet binary: %w", err)
		}
		s.Logger.Printf("binary SHA256: %s", checksum)
		if err := verifyELF(s.BinaryPath); err != nil {
			os.Remove(s.BinaryPath)
			return fmt.Errorf("downloaded binary is invalid: %w", err)
		}
		if err := os.Chmod(s.BinaryPath, 0o755); err != nil {
			return fmt.Errorf("chmod parakeet binary: %w", err)
		}
	}

	// Download models
	models := modelFileURLs()
	for filename, url := range models {
		dest := filepath.Join(s.ModelsDir, filename)
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			s.Logger.Printf("downloading model file: %s", filename)
			if _, err := downloadFile(url, dest, progress, filename); err != nil {
				return fmt.Errorf("download model %s: %w", filename, err)
			}
		}
	}

	// Download ONNX Runtime if not available
	if !s.onnxRuntimeAvailable() {
		s.Logger.Printf("downloading ONNX Runtime %s...", onnxRuntimeVersion)
		if err := downloadAndExtractOnnxRuntime(s.OnnxDir, progress); err != nil {
			return fmt.Errorf("download onnxruntime: %w", err)
		}
	}

	return nil
}

// Start spawns the Parakeet server process and waits for it to become healthy.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		return fmt.Errorf("server already running (pid %d)", s.cmd.Process.Pid)
	}

	s.Logger.Printf("starting parakeet on port %d", s.Port)

	cmd := exec.CommandContext(ctx, s.BinaryPath,
		"-port", fmt.Sprintf("%d", s.Port),
		"-models", s.ModelsDir,
	)
	cmd.Stdout = s.Logger.Writer()
	cmd.Stderr = s.Logger.Writer()

	// Set ONNXRUNTIME_LIB so parakeet can find bundled ONNX Runtime,
	// and LD_LIBRARY_PATH as fallback for dynamic linker resolution.
	onnxLib := filepath.Join(s.OnnxDir, "libonnxruntime.so")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("ONNXRUNTIME_LIB=%s", onnxLib),
		fmt.Sprintf("LD_LIBRARY_PATH=%s:%s", s.OnnxDir, os.Getenv("LD_LIBRARY_PATH")),
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start parakeet: %w", err)
	}
	s.cmd = cmd

	// Wait for server to become healthy (up to 120s for model loading)
	healthURL := fmt.Sprintf("http://localhost:%d/v1/models", s.Port)
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				s.Logger.Printf("parakeet is ready")
				return nil
			}
		}
	}

	return fmt.Errorf("parakeet did not become healthy within 120s")
}

// Stop sends SIGTERM to the server process and waits for it to exit.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	s.Logger.Printf("stopping parakeet (pid %d)", s.cmd.Process.Pid)

	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		// Process may have already exited
		s.Logger.Printf("signal error (may be already stopped): %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.cmd.Process.Kill()
		<-done
	}

	s.cmd = nil
	return nil
}

// Running returns true if the server process is alive.
func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	// Check if process is still running by sending signal 0
	return s.cmd.Process.Signal(syscall.Signal(0)) == nil
}

// Restart stops and then starts the server.
func (s *Server) Restart(ctx context.Context) error {
	if err := s.Stop(); err != nil {
		s.Logger.Printf("stop error during restart: %v", err)
	}
	return s.Start(ctx)
}
