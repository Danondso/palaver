package transcriber

import (
	"context"
	"fmt"
	"testing"
)

func TestCommandTranscribe(t *testing.T) {
	transcriber := NewCommand("echo hello from palaver", 30, nil)
	result, err := transcriber.Transcribe(context.Background(), []byte("fake-wav"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello from palaver" {
		t.Errorf("expected 'hello from palaver', got %q", result)
	}
}

func TestCommandTranscribeWithInputSubstitution(t *testing.T) {
	// Use cat to read the temp file back — verifies {input} substitution works
	transcriber := NewCommand("cat {input}", 30, nil)
	wavData := []byte("test-wav-content")
	result, err := transcriber.Transcribe(context.Background(), wavData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test-wav-content" {
		t.Errorf("expected 'test-wav-content', got %q", result)
	}
}

func TestCommandTranscribeBadCommand(t *testing.T) {
	transcriber := NewCommand("nonexistent-binary-xyz {input}", 30, nil)
	_, err := transcriber.Transcribe(context.Background(), []byte("data"))
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestNewTranscriberFactory(t *testing.T) {
	t.Run("openai", func(t *testing.T) {
		cfg := &testTranscriptionConfig{provider: "openai", baseURL: "http://localhost:5092", model: "default", timeoutSec: 30}
		_, err := newTranscriberFromValues(cfg.provider, cfg.baseURL, cfg.model, cfg.timeoutSec, cfg.command)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("command", func(t *testing.T) {
		cfg := &testTranscriptionConfig{provider: "command", command: "echo {input}", timeoutSec: 30}
		_, err := newTranscriberFromValues(cfg.provider, cfg.baseURL, cfg.model, cfg.timeoutSec, cfg.command)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := newTranscriberFromValues("unknown", "", "", 30, "")
		if err == nil {
			t.Error("expected error for unknown provider")
		}
	})
}

type testTranscriptionConfig struct {
	provider   string
	baseURL    string
	model      string
	timeoutSec int
	command    string
}

func newTranscriberFromValues(provider, baseURL, model string, timeoutSec int, command string) (Transcriber, error) {
	switch provider {
	case "openai":
		return NewOpenAI(baseURL, model, timeoutSec, nil), nil
	case "command":
		if command == "" {
			return nil, fmt.Errorf("command provider requires a non-empty command")
		}
		return NewCommand(command, timeoutSec, nil), nil
	default:
		return nil, fmt.Errorf("unknown transcription provider: %s", provider)
	}
}
