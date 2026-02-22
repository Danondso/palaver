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
		{"newline", "hello\nworld", `hello\nworld`},
		{"carriage return", "hello\rworld", `hello\rworld`},
		{"tab", "hello\tworld", `hello\tworld`},
		{"backspace stripped", "hello\bworld", "helloworld"},
		{"multiline with quotes", "line1\nline2 \"quoted\"\nline3", `line1\nline2 \"quoted\"\nline3`},
		{"injection attempt", `hello" & do shell script "rm -rf ~" & "`, `hello\" & do shell script \"rm -rf ~\" & \"`},
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
	t.Skip("requires Accessibility permissions in System Settings > Privacy & Security")
}
