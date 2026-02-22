package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Danondso/palaver/internal/config"
)

// Server manages the lifecycle of a managed transcription server.
type Server struct {
	BinaryPath string
	ModelsDir  string
	OnnxDir    string // directory containing libonnxruntime (Linux only)
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
		BinaryPath: resolveServerBinary(dataDir),
		ModelsDir:  filepath.Join(dataDir, "models"),
		OnnxDir:    filepath.Join(dataDir, "onnxruntime"),
		Port:       cfg.Port,
		Logger:     logger,
	}
}

// IsInstalled returns true if the server binary and required model files exist.
func (s *Server) IsInstalled() bool {
	if !isServerInstalled(s.BinaryPath, s.ModelsDir) {
		return false
	}
	if needsOnnxRuntime() && !s.onnxRuntimeAvailable() {
		return false
	}
	return true
}

// onnxRuntimeAvailable checks if ONNX Runtime is available either system-wide
// or in our bundled OnnxDir.
func (s *Server) onnxRuntimeAvailable() bool {
	// Check bundled copy first
	matches, _ := filepath.Glob(filepath.Join(s.OnnxDir, "libonnxruntime"+libExtension()+"*"))
	if len(matches) > 0 {
		return true
	}
	// Check if it's available system-wide
	return systemOnnxRuntimeAvailable()
}

// Setup downloads server dependencies if they are missing.
func (s *Server) Setup(progress ProgressFunc) error {
	return setupServer(s.BinaryPath, s.ModelsDir, s.OnnxDir, s.Logger, progress)
}

// Start spawns the server process and waits for it to become healthy.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		return fmt.Errorf("server already running (pid %d)", s.cmd.Process.Pid)
	}

	s.Logger.Printf("starting %s on port %d", serverBinaryName(), s.Port)

	cmd := exec.CommandContext(ctx, s.BinaryPath, serverArgs(s.Port, s.ModelsDir)...) //nolint:gosec // binary path from managed install
	cmd.Stdout = s.Logger.Writer()
	cmd.Stderr = s.Logger.Writer()

	env := serverEnv(s.OnnxDir)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", serverBinaryName(), err)
	}
	s.cmd = cmd

	// Wait for server to become healthy
	healthURL := healthCheckURL(s.Port)
	timeout := healthCheckTimeout()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		resp, err := http.Get(healthURL) //nolint:gosec // URL from hardcoded localhost
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				s.Logger.Printf("%s", serverReadyLog())
				return nil
			}
		}
	}

	return fmt.Errorf("%s did not become healthy within %s", serverBinaryName(), timeout)
}

// Stop sends SIGTERM to the server process and waits for it to exit.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	s.Logger.Printf("stopping %s (pid %d)", serverBinaryName(), s.cmd.Process.Pid)

	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		// Process may have already exited
		s.Logger.Printf("signal error (may be already stopped): %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
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
