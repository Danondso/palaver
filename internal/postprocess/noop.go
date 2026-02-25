package postprocess

import "context"

// NoopPostProcessor returns text unchanged.
type NoopPostProcessor struct{}

func (n *NoopPostProcessor) Rewrite(_ context.Context, text string) (string, error) {
	return text, nil
}
