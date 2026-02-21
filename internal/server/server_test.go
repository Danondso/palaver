package server

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/Danondso/palaver/internal/config"
)

func TestNewResolvesDefaultDataDir(t *testing.T) {
	cfg := &config.ServerConfig{Port: 5092}
	logger := log.New(io.Discard, "", 0)
	srv := New(cfg, logger)

	expected := config.DefaultDataDir()
	if filepath.Dir(srv.BinaryPath) != expected {
		t.Errorf("BinaryPath parent = %q, want %q", filepath.Dir(srv.BinaryPath), expected)
	}
	if filepath.Dir(srv.ModelsDir) != expected {
		t.Errorf("ModelsDir parent = %q, want %q", filepath.Dir(srv.ModelsDir), expected)
	}
}

func TestNewUsesCustomDataDir(t *testing.T) {
	cfg := &config.ServerConfig{DataDir: "/tmp/palaver-test", Port: 9999}
	logger := log.New(io.Discard, "", 0)
	srv := New(cfg, logger)

	if srv.BinaryPath != "/tmp/palaver-test/parakeet" {
		t.Errorf("BinaryPath = %q, want /tmp/palaver-test/parakeet", srv.BinaryPath)
	}
	if srv.ModelsDir != "/tmp/palaver-test/models" {
		t.Errorf("ModelsDir = %q, want /tmp/palaver-test/models", srv.ModelsDir)
	}
	if srv.Port != 9999 {
		t.Errorf("Port = %d, want 9999", srv.Port)
	}
}

func TestIsInstalledFalseWhenMissing(t *testing.T) {
	cfg := &config.ServerConfig{DataDir: "/tmp/palaver-nonexistent-" + t.Name(), Port: 5092}
	logger := log.New(io.Discard, "", 0)
	srv := New(cfg, logger)

	if srv.IsInstalled() {
		t.Error("IsInstalled() = true for nonexistent paths")
	}
}

func TestIsInstalledTrueWhenPresent(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.ServerConfig{DataDir: dir, Port: 5092}
	logger := log.New(io.Discard, "", 0)
	srv := New(cfg, logger)

	// Create the binary
	if err := os.WriteFile(srv.BinaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create the model dir and encoder file
	if err := os.MkdirAll(srv.ModelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srv.ModelsDir, "encoder-model.int8.onnx"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create the onnxruntime dir and lib
	if err := os.MkdirAll(srv.OnnxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srv.OnnxDir, "libonnxruntime.so"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !srv.IsInstalled() {
		t.Error("IsInstalled() = false when binary, model, and onnxruntime exist")
	}
}

func TestRunningFalseWhenNotStarted(t *testing.T) {
	cfg := &config.ServerConfig{DataDir: t.TempDir(), Port: 5092}
	logger := log.New(io.Discard, "", 0)
	srv := New(cfg, logger)

	if srv.Running() {
		t.Error("Running() = true when server not started")
	}
}

func TestStopNoopWhenNotRunning(t *testing.T) {
	cfg := &config.ServerConfig{DataDir: t.TempDir(), Port: 5092}
	logger := log.New(io.Discard, "", 0)
	srv := New(cfg, logger)

	if err := srv.Stop(); err != nil {
		t.Errorf("Stop() on idle server: %v", err)
	}
}
