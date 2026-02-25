package postprocess

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Danondso/palaver/internal/config"
)

// saveToneState saves and returns a restore function for global tone state.
func saveToneState() func() {
	origOrder := make([]string, len(toneOrder))
	copy(origOrder, toneOrder)
	origTones := make(map[string]Tone, len(tones))
	for k, v := range tones {
		origTones[k] = v
	}
	return func() {
		toneOrder = origOrder
		tones = origTones
	}
}

func TestNoopRewritePassthrough(t *testing.T) {
	pp := &NoopPostProcessor{}
	text := "Hello, how are you?"
	result, err := pp.Rewrite(context.Background(), text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != text {
		t.Errorf("expected %q, got %q", text, result)
	}
}

func TestLLMRewrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "llama3.2" {
			t.Errorf("expected model llama3.2, got %s", req.Model)
		}
		if len(req.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("expected system role, got %s", req.Messages[0].Role)
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("expected user role, got %s", req.Messages[1].Role)
		}

		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "Could you please help me?"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	pp := NewLLM(srv.URL, "llama3.2", "be polite", 10, log.New(io.Discard, "", 0))
	result, err := pp.Rewrite(context.Background(), "help me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Could you please help me?" {
		t.Errorf("expected rewritten text, got %q", result)
	}
}

func TestLLMRewriteServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	pp := NewLLM(srv.URL, "llama3.2", "be polite", 10, log.New(io.Discard, "", 0))
	_, err := pp.Rewrite(context.Background(), "help me")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestLLMRewriteEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse{})
	}))
	defer srv.Close()

	pp := NewLLM(srv.URL, "llama3.2", "be polite", 10, log.New(io.Discard, "", 0))
	_, err := pp.Rewrite(context.Background(), "help me")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestLLMRewriteTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pp := NewLLM(srv.URL, "llama3.2", "be polite", 1, log.New(io.Discard, "", 0))
	_, err := pp.Rewrite(context.Background(), "help me")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestLLMRewriteMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	pp := NewLLM(srv.URL, "llama3.2", "be polite", 10, log.New(io.Discard, "", 0))
	_, err := pp.Rewrite(context.Background(), "help me")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestLLMListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{
				{"id": "llama3.2"},
				{"id": "mistral"},
			},
		})
	}))
	defer srv.Close()

	pp := NewLLM(srv.URL, "llama3.2", "be polite", 10, log.New(io.Discard, "", 0))
	models, err := pp.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0] != "llama3.2" {
		t.Errorf("expected llama3.2, got %s", models[0])
	}
	if models[1] != "mistral" {
		t.Errorf("expected mistral, got %s", models[1])
	}
}

func TestLLMListModelsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	pp := NewLLM(srv.URL, "llama3.2", "be polite", 10, log.New(io.Discard, "", 0))
	_, err := pp.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error for unavailable service")
	}
}

func TestResolveToneBuiltin(t *testing.T) {
	defer saveToneState()()

	for _, name := range []string{"off", "polite", "formal", "casual", "direct", "token-efficient"} {
		tone := ResolveTone(name)
		if tone.Name != name {
			t.Errorf("ResolveTone(%q): expected name %q, got %q", name, name, tone.Name)
		}
	}
}

func TestResolveToneUnknown(t *testing.T) {
	defer saveToneState()()

	tone := ResolveTone("nonexistent")
	if tone.Name != "off" {
		t.Errorf("expected fallback to off, got %s", tone.Name)
	}
}

func TestNextToneCycle(t *testing.T) {
	defer saveToneState()()

	expected := []string{"polite", "formal", "casual", "direct", "token-efficient", "off"}
	current := "off"
	for _, exp := range expected {
		next := NextTone(current)
		if next != exp {
			t.Errorf("NextTone(%q): expected %q, got %q", current, exp, next)
		}
		current = next
	}
}

func TestNextToneUnknown(t *testing.T) {
	defer saveToneState()()

	next := NextTone("nonexistent")
	if next != "off" {
		t.Errorf("expected off for unknown tone, got %s", next)
	}
}

func TestRegisterCustomTones(t *testing.T) {
	defer saveToneState()()

	custom := []config.CustomTone{
		{Name: "pirate", Prompt: "Arr, rewrite as a pirate."},
	}
	RegisterCustomTones(custom, log.New(io.Discard, "", 0))

	tone := ResolveTone("pirate")
	if tone.Name != "pirate" {
		t.Errorf("expected pirate, got %s", tone.Name)
	}
	if tone.Prompt != "Arr, rewrite as a pirate." {
		t.Errorf("unexpected prompt: %s", tone.Prompt)
	}

	names := ToneNames()
	found := false
	for _, n := range names {
		if n == "pirate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected pirate in tone cycle order")
	}
}

func TestRegisterCustomToneOverridesBuiltin(t *testing.T) {
	defer saveToneState()()

	custom := []config.CustomTone{
		{Name: "polite", Prompt: "Custom polite prompt."},
	}
	RegisterCustomTones(custom, log.New(io.Discard, "", 0))

	tone := ResolveTone("polite")
	if tone.Prompt != "Custom polite prompt." {
		t.Errorf("expected custom prompt, got %s", tone.Prompt)
	}
}

func TestRegisterCustomToneEmptyName(t *testing.T) {
	defer saveToneState()()

	before := len(ToneNames())
	custom := []config.CustomTone{
		{Name: "", Prompt: "should be skipped"},
	}
	RegisterCustomTones(custom, log.New(io.Discard, "", 0))

	if len(ToneNames()) != before {
		t.Error("empty name tone should not be registered")
	}
}

func TestNewFactoryNoop(t *testing.T) {
	defer saveToneState()()

	cfg := &config.PostProcessingConfig{Enabled: false, Tone: "off"}
	pp := New(cfg, nil, log.New(io.Discard, "", 0))
	if _, ok := pp.(*NoopPostProcessor); !ok {
		t.Errorf("expected NoopPostProcessor when disabled, got %T", pp)
	}
}

func TestNewFactoryNoopWhenToneOff(t *testing.T) {
	defer saveToneState()()

	cfg := &config.PostProcessingConfig{Enabled: true, Tone: "off"}
	pp := New(cfg, nil, log.New(io.Discard, "", 0))
	if _, ok := pp.(*NoopPostProcessor); !ok {
		t.Errorf("expected NoopPostProcessor when tone is off, got %T", pp)
	}
}

func TestNewFactoryLLM(t *testing.T) {
	defer saveToneState()()

	cfg := &config.PostProcessingConfig{
		Enabled:    true,
		Tone:       "formal",
		Model:      "llama3.2",
		BaseURL:    "http://localhost:11434/v1",
		TimeoutSec: 10,
	}
	pp := New(cfg, nil, log.New(io.Discard, "", 0))
	if _, ok := pp.(*LLMPostProcessor); !ok {
		t.Errorf("expected LLMPostProcessor when enabled, got %T", pp)
	}
}
