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
	"formal":          {Name: "formal", Prompt: "You are a post-processor for speech-to-text transcription. Rewrite the transcribed text in a professional, formal tone suitable for business communication. Remove filler words and false starts. Preserve all specific terms, names, technical words, and instructions exactly as spoken. Return only the rewritten text."},
	"direct":          {Name: "direct", Prompt: "You are a post-processor for speech-to-text transcription. Rewrite the transcribed text to be concise and direct. Remove all filler words (um, uh, like, you know, so, basically, actually, I mean, kind of, sort of), false starts, and redundant phrasing. Preserve all specific terms, names, technical words, and instructions exactly as spoken. Return only the rewritten text."},
	"token-efficient": {Name: "token-efficient", Prompt: "You are a post-processor for speech-to-text transcription. Compress the transcribed speech into concise text while preserving the speaker's original intent and meaning. Rules: 1) Remove ALL filler words, hedging, false starts, and conversational padding. 2) Use imperative form where the speaker is giving commands. 3) Strip unnecessary articles, pronouns, and linking phrases. 4) If the speaker listed steps or numbered instructions, preserve that structure. 5) Preserve all technical terms, names, code references, and specific values exactly. 6) Do NOT add information, steps, or details the speaker did not say. 7) Do NOT interpret or expand on what the speaker meant. Return only the compressed text."},
}

var builtinToneNames = map[string]bool{
	"off": true, "formal": true,
	"direct": true, "token-efficient": true,
}

var toneOrder = []string{"off", "formal", "direct", "token-efficient"}

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
	toneOrder = []string{"off", "formal", "direct", "token-efficient"}
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
