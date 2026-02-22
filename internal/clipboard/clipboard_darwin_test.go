//go:build darwin

package clipboard

import "testing"

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no escaping", "hello world", "hello world"},
		{"double quotes", `say "hello"`, `say \"hello\"`},
		{"backslash", `path\to\file`, `path\\to\\file`},
		{"both", `"hello\world"`, `\"hello\\world\"`},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeAppleScript(tt.input)
			if result != tt.expected {
				t.Errorf("escapeAppleScript(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPasteTextRequiresAccessibility(t *testing.T) {
	t.Log("clipboard.PasteText requires Accessibility permissions for full testing")
}
