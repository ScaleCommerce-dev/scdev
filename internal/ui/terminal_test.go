package ui

import (
	"os"
	"testing"
)

func TestPlainMode(t *testing.T) {
	// Save original env
	origEnv := os.Getenv("SCDEV_PLAIN")
	defer os.Setenv("SCDEV_PLAIN", origEnv)

	tests := []struct {
		name        string
		envValue    string
		configPlain bool
		expected    bool
	}{
		{"env 1 overrides config false", "1", false, true},
		{"env true overrides config false", "true", false, true},
		{"env 0 is not plain", "0", false, false},
		{"env empty uses config true", "", true, true},
		{"env empty uses config false", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SCDEV_PLAIN", tt.envValue)
			result := PlainMode(tt.configPlain)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHyperlink(t *testing.T) {
	url := "http://example.com"
	text := "Example"

	// Plain mode should return just text
	result := Hyperlink(url, text, true)
	if result != text {
		t.Errorf("plain mode: expected %q, got %q", text, result)
	}

	// Empty text should use URL
	result = Hyperlink(url, "", true)
	if result != url {
		t.Errorf("empty text plain mode: expected %q, got %q", url, result)
	}
}

func TestStatusColor(t *testing.T) {
	// Plain mode should return uncolored text
	statuses := []string{"running", "stopped", "not created", "unknown"}
	for _, status := range statuses {
		result := StatusColor(status, true)
		if result != status {
			t.Errorf("plain mode status %q: expected %q, got %q", status, status, result)
		}
	}
}

func TestColor(t *testing.T) {
	text := "test"

	// Plain mode should return uncolored text
	result := Color(text, "green", true)
	if result != text {
		t.Errorf("plain mode: expected %q, got %q", text, result)
	}

	// Unknown color should return uncolored text
	result = Color(text, "unknown", false)
	if result != text {
		t.Errorf("unknown color: expected %q, got %q", text, result)
	}
}

func TestHyperlinkKeyHint(t *testing.T) {
	// Plain mode should return empty string
	result := HyperlinkKeyHint(true)
	if result != "" {
		t.Errorf("plain mode: expected empty string, got %q", result)
	}
}
