package transcriber

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Command implements Transcriber by shelling out to an external command.
type Command struct {
	command    string
	timeoutSec int
	logger     *log.Logger
}

// NewCommand creates a command-based transcriber.
// The command string should contain {input} which will be replaced with
// the path to a temporary WAV file.
func NewCommand(command string, timeoutSec int, logger *log.Logger) *Command {
	return &Command{
		command:    command,
		timeoutSec: timeoutSec,
		logger:     logger,
	}
}

// Transcribe writes WAV data to a temp file, runs the configured command,
// and returns stdout as the transcript.
func (c *Command) Transcribe(ctx context.Context, wavData []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.timeoutSec)*time.Second)
	defer cancel()

	tmpFile, err := os.CreateTemp("", "palaver-*.wav")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(wavData); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	_ = tmpFile.Close()

	cmdStr := strings.ReplaceAll(c.command, "{input}", tmpPath)
	if cmdStr == "" {
		return "", fmt.Errorf("empty command after substitution")
	}

	if c.logger != nil {
		c.logger.Printf("transcribe command: %s wav_size=%d", cmdStr, len(wavData))
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	output, err := cmd.Output()
	latency := time.Since(start)
	if err != nil {
		return "", fmt.Errorf("run command: %w", err)
	}

	text := strings.TrimSpace(string(output))
	if c.logger != nil {
		c.logger.Printf("transcribe response: output_size=%d latency=%s", len(output), latency.Round(time.Millisecond))
		c.logger.Printf("transcribe result: %q", text)
	}
	return text, nil
}
