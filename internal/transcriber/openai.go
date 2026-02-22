package transcriber

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// OpenAI implements Transcriber using the OpenAI-compatible
// POST /v1/audio/transcriptions endpoint.
type OpenAI struct {
	baseURL    string
	model      string
	timeoutSec int
	client     *http.Client
	logger     *log.Logger
}

// NewOpenAI creates an OpenAI-compatible transcriber.
func NewOpenAI(baseURL, model string, timeoutSec int, tlsSkipVerify bool, logger *log.Logger) *OpenAI {
	client := &http.Client{}
	if tlsSkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // user-configured opt-in for self-signed certs
		}
	}
	return &OpenAI{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		timeoutSec: timeoutSec,
		client:     client,
		logger:     logger,
	}
}

// ConfiguredModel returns the model name from config.
func (o *OpenAI) ConfiguredModel() string {
	return o.model
}

// Ping checks if the transcription backend is reachable.
func (o *OpenAI) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"/", nil)
	if err != nil {
		return fmt.Errorf("build ping request: %w", err)
	}

	resp, err := o.client.Do(req) //nolint:gosec // URL from user config
	if err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	_ = resp.Body.Close()
	return nil
}

// ListModels queries the /v1/models endpoint and returns available model IDs.
func (o *OpenAI) ListModels(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("build models request: %w", err)
	}

	resp, err := o.client.Do(req) //nolint:gosec // URL from user config
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list models: status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	models := make([]string, len(result.Data))
	for i, m := range result.Data {
		models[i] = m.ID
	}
	return models, nil
}

// Transcribe sends WAV data to the OpenAI-compatible endpoint and returns the text.
func (o *OpenAI) Transcribe(ctx context.Context, wavData []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(o.timeoutSec)*time.Second)
	defer cancel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(wavData); err != nil {
		return "", fmt.Errorf("write wav data: %w", err)
	}

	if err := writer.WriteField("model", o.model); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}
	if err := writer.WriteField("response_format", "text"); err != nil {
		return "", fmt.Errorf("write response_format field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	url := o.baseURL + "/v1/audio/transcriptions"
	if o.logger != nil {
		o.logger.Printf("transcribe request: POST %s wav_size=%d", url, len(wavData))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	start := time.Now()
	resp, err := o.client.Do(req) //nolint:gosec // URL from user config
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	latency := time.Since(start)

	if o.logger != nil {
		o.logger.Printf("transcribe response: status=%d body_size=%d latency=%s", resp.StatusCode, len(respBody), latency.Round(time.Millisecond))
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("transcription failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	text := strings.TrimSpace(string(respBody))
	if o.logger != nil {
		o.logger.Printf("transcribe result: %q", text)
	}
	return text, nil
}
