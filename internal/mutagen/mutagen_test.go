package mutagen

import (
	"testing"
)

func TestNew(t *testing.T) {
	m := New("/usr/local/bin/mutagen")

	if m == nil {
		t.Fatal("expected non-nil Mutagen")
	}

	if m.binaryPath != "/usr/local/bin/mutagen" {
		t.Errorf("expected binaryPath '/usr/local/bin/mutagen', got %q", m.binaryPath)
	}
}

func TestBinaryPath(t *testing.T) {
	m := New("/custom/path/to/mutagen")

	path := m.BinaryPath()
	if path != "/custom/path/to/mutagen" {
		t.Errorf("expected '/custom/path/to/mutagen', got %q", path)
	}
}

func TestMergeIgnores(t *testing.T) {
	tests := []struct {
		name        string
		userIgnores []string
		wantLen     int
		wantHas     []string
	}{
		{
			name:        "empty user ignores",
			userIgnores: nil,
			wantLen:     len(BuiltinIgnores),
			wantHas:     BuiltinIgnores,
		},
		{
			name:        "user ignores added",
			userIgnores: []string{"vendor", "node_modules"},
			wantLen:     len(BuiltinIgnores) + 2,
			wantHas:     append(BuiltinIgnores, "vendor", "node_modules"),
		},
		{
			name:        "duplicate builtin ignored",
			userIgnores: []string{".git", "custom"},
			wantLen:     len(BuiltinIgnores) + 1, // .git is duplicate, only custom added
			wantHas:     append(BuiltinIgnores, "custom"),
		},
		{
			name:        "user duplicates deduplicated",
			userIgnores: []string{"vendor", "vendor", "node_modules"},
			wantLen:     len(BuiltinIgnores) + 2, // vendor only counted once
			wantHas:     append(BuiltinIgnores, "vendor", "node_modules"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeIgnores(tt.userIgnores)

			if len(result) != tt.wantLen {
				t.Errorf("MergeIgnores() returned %d items, want %d", len(result), tt.wantLen)
			}

			// Check that all expected items are present
			resultSet := make(map[string]bool)
			for _, r := range result {
				resultSet[r] = true
			}

			for _, want := range tt.wantHas {
				if !resultSet[want] {
					t.Errorf("MergeIgnores() missing expected item %q", want)
				}
			}
		})
	}
}

func TestMergeIgnores_OrderPreserved(t *testing.T) {
	result := MergeIgnores([]string{"custom1", "custom2"})

	// Built-in ignores should come first
	for i, builtin := range BuiltinIgnores {
		if i >= len(result) {
			t.Fatalf("result too short, expected at least %d items", len(BuiltinIgnores))
		}
		if result[i] != builtin {
			t.Errorf("expected builtin %q at index %d, got %q", builtin, i, result[i])
		}
	}

	// User ignores should follow
	if result[len(BuiltinIgnores)] != "custom1" {
		t.Errorf("expected 'custom1' after builtins, got %q", result[len(BuiltinIgnores)])
	}
	if result[len(BuiltinIgnores)+1] != "custom2" {
		t.Errorf("expected 'custom2' after custom1, got %q", result[len(BuiltinIgnores)+1])
	}
}

func TestBuiltinIgnores(t *testing.T) {
	// Verify expected built-in ignores are present
	expected := []string{".git", ".DS_Store"}

	if len(BuiltinIgnores) != len(expected) {
		t.Errorf("expected %d builtin ignores, got %d", len(expected), len(BuiltinIgnores))
	}

	for i, want := range expected {
		if BuiltinIgnores[i] != want {
			t.Errorf("expected BuiltinIgnores[%d] = %q, got %q", i, want, BuiltinIgnores[i])
		}
	}
}

func TestSessionConfig(t *testing.T) {
	cfg := SessionConfig{
		Name:    "scdev-myproject-app",
		Alpha:   "/Users/test/myproject",
		Beta:    "docker://app.myproject.scdev/app",
		Ignores: []string{".git", "vendor"},
	}

	if cfg.Name != "scdev-myproject-app" {
		t.Errorf("expected Name 'scdev-myproject-app', got %q", cfg.Name)
	}
	if cfg.Alpha != "/Users/test/myproject" {
		t.Errorf("expected Alpha '/Users/test/myproject', got %q", cfg.Alpha)
	}
	if cfg.Beta != "docker://app.myproject.scdev/app" {
		t.Errorf("expected Beta 'docker://app.myproject.scdev/app', got %q", cfg.Beta)
	}
	if len(cfg.Ignores) != 2 {
		t.Errorf("expected 2 ignores, got %d", len(cfg.Ignores))
	}
}
