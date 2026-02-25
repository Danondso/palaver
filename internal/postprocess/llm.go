package postprocess

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// LLMPostProcessor rewrites text via an OpenAI-compatible chat completions API.
type LLMPostProcessor struct {
	baseURL    string
	model      string
	prompt     string
	timeoutSec int
	client     *http.Client
	logger     *log.Logger
}

// NewLLM creates an LLM-based post-processor.
func NewLLM(baseURL, model, prompt string, timeoutSec int, logger *log.Logger) *LLMPostProcessor {
	return &LLMPostProcessor{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		prompt:     prompt,
		timeoutSec: timeoutSec,
		client:     &http.Client{},
		logger:     logger,
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Rewrite sends the text to the LLM with the tone prompt and returns the rewritten text.
func (l *LLMPostProcessor) Rewrite(ctx context.Context, text string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(l.timeoutSec)*time.Second)
	defer cancel()

	reqBody := chatRequest{
		Model: l.model,
		Messages: []chatMessage{
			{Role: "system", Content: l.prompt},
			{Role: "user", Content: text},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := l.baseURL + "/chat/completions"
	if l.logger != nil {
		l.logger.Printf("postprocess: POST %s model=%s text_len=%d", url, l.model, len(text))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB cap
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	latency := time.Since(start)

	if l.logger != nil {
		l.logger.Printf("postprocess: status=%d body_size=%d latency=%s", resp.StatusCode, len(respBody), latency.Round(time.Millisecond))
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("post-processing failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	result := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if l.logger != nil {
		l.logger.Printf("postprocess result: %q", result)
	}
	return result, nil
}

// ListModels queries GET {baseURL}/models and returns available model IDs.
func (l *LLMPostProcessor) ListModels(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("build models request: %w", err)
	}

	resp, err := l.client.Do(req)
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
