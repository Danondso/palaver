package postprocess

import (
	"context"
	"log"
	"strings"

	"github.com/Danondso/palaver/internal/config"
)

// PostProcessor rewrites transcribed text according to a tone preset.
type PostProcessor interface {
	Rewrite(ctx context.Context, text string) (string, error)
}

// ModelLister is optionally implemented by post-processors that can
// list available models from the backend.
type ModelLister interface {
	ListModels(ctx context.Context) ([]string, error)
}

// Tone holds a tone name and its system prompt.
type Tone struct {
	Name   string
	Prompt string
}

var builtinTones = map[string]Tone{
	"off":             {Name: "off", Prompt: ""},
	"polite":          {Name: "polite", Prompt: "Rewrite the following transcribed speech to be more polite, adding please and thank you where appropriate. Keep the meaning identical. Return only the rewritten text, no explanation."},
	"formal":          {Name: "formal", Prompt: "Rewrite the following transcribed speech in a professional, formal tone suitable for business communication. Keep the meaning identical. Return only the rewritten text, no explanation."},
	"casual":          {Name: "casual", Prompt: "Rewrite the following transcribed speech in a relaxed, casual tone. Keep the meaning identical. Return only the rewritten text, no explanation."},
	"direct":          {Name: "direct", Prompt: "Rewrite the following transcribed speech to be concise and direct. Remove all filler words (um, uh, like, you know, so, basically, actually, I mean, kind of, sort of) and unnecessary phrasing. Preserve the core meaning. Return only the rewritten text, no explanation."},
	"token-efficient": {Name: "token-efficient", Prompt: "Rewrite the following transcribed speech to be maximally concise. First, remove ALL filler and hesitation words (um, uh, like, you know, so, basically, I mean, etc). Then compress: strip articles, pronouns, and redundant words where possible. Use imperative form. The result should be significantly shorter than the input. Return only the rewritten text, no explanation."},
}

var builtinToneNames = map[string]bool{
	"off": true, "polite": true, "formal": true,
	"casual": true, "direct": true, "token-efficient": true,
}

var toneOrder = []string{"off", "polite", "formal", "casual", "direct", "token-efficient"}

var tones map[string]Tone

func init() {
	resetTones()
}

func resetTones() {
	tones = make(map[string]Tone, len(builtinTones))
	for k, v := range builtinTones {
		tones[k] = v
	}
}

// ResetTones restores the tone registry to its built-in defaults.
// Intended for use in tests to prevent state leaking between test cases.
func ResetTones() {
	resetTones()
	toneOrder = []string{"off", "polite", "formal", "casual", "direct", "token-efficient"}
}

// RegisterCustomTones adds custom tones to the tone map and cycle order.
// Custom tones with the same name as a built-in tone take precedence;
// a log message is emitted to inform the user.
func RegisterCustomTones(custom []config.CustomTone, logger *log.Logger) {
	for _, ct := range custom {
		key := strings.ToLower(ct.Name)
		if key == "" {
			continue
		}
		if builtinToneNames[key] && logger != nil {
			logger.Printf("custom tone %q overrides built-in default", key)
		}
		tones[key] = Tone{Name: ct.Name, Prompt: ct.Prompt}
		found := false
		for _, name := range toneOrder {
			if name == key {
				found = true
				break
			}
		}
		if !found {
			toneOrder = append(toneOrder, key)
		}
	}
}

// ToneNames returns a copy of the tone cycle order.
func ToneNames() []string {
	out := make([]string, len(toneOrder))
	copy(out, toneOrder)
	return out
}

// ResolveTone returns the Tone for the given name, or the "off" tone if not found.
func ResolveTone(name string) Tone {
	if t, ok := tones[strings.ToLower(name)]; ok {
		return t
	}
	return tones["off"]
}

// NextTone returns the tone name after the given one in the cycle order.
func NextTone(current string) string {
	current = strings.ToLower(current)
	for i, name := range toneOrder {
		if name == current {
			return toneOrder[(i+1)%len(toneOrder)]
		}
	}
	return toneOrder[0]
}

// New creates a PostProcessor based on the config.
// If tone is "off" or post-processing is disabled, returns a NoopPostProcessor.
func New(cfg *config.PostProcessingConfig, customTones []config.CustomTone, logger *log.Logger) PostProcessor {
	RegisterCustomTones(customTones, logger)
	if !cfg.Enabled || strings.ToLower(cfg.Tone) == "off" {
		return &NoopPostProcessor{}
	}
	tone := ResolveTone(cfg.Tone)
	if tone.Prompt == "" {
		return &NoopPostProcessor{}
	}
	return NewLLM(cfg.BaseURL, cfg.Model, tone.Prompt, cfg.TimeoutSec, logger)
}
