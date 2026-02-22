package hotkey

import "context"

// Listener listens for global hotkey press/release events.
type Listener interface {
	Start(ctx context.Context, onDown func(), onUp func()) error
	Stop()
	KeyName() string
}
