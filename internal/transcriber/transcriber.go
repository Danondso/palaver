package transcriber

import (
	"context"
	"fmt"
	"log"

	"github.com/Danondso/palaver/internal/config"
)

// Transcriber transcribes WAV audio data to text.
type Transcriber interface {
	Transcribe(ctx context.Context, wavData []byte) (string, error)
}

// HealthChecker is optionally implemented by transcribers that can report
// backend availability.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// ModelLister is optionally implemented by transcribers that can report
// which models are available on the backend.
type ModelLister interface {
	ListModels(ctx context.Context) ([]string, error)
}

// ConfiguredModeler is optionally implemented by transcribers that can report
// the configured model name as a fallback when ListModels is unavailable.
type ConfiguredModeler interface {
	ConfiguredModel() string
}

// New creates a Transcriber based on the provider config.
func New(cfg *config.TranscriptionConfig, logger *log.Logger) (Transcriber, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAI(cfg.BaseURL, cfg.Model, cfg.TimeoutSec, cfg.TLSSkipVerify, logger), nil
	case "command":
		if cfg.Command == "" {
			return nil, fmt.Errorf("command provider requires a non-empty command")
		}
		return NewCommand(cfg.Command, cfg.TimeoutSec, logger), nil
	default:
		return nil, fmt.Errorf("unknown transcription provider: %s", cfg.Provider)
	}
}
