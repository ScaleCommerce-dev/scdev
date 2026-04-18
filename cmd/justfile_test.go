package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/ScaleCommerce-DEV/scdev/internal/tools"
)

// ensureJust returns a *tools.Just backed by a locally installed just
// binary. Skips the test if just isn't available on PATH - integration
// tests cover the download path.
func ensureJust(t *testing.T) *tools.Just {
	t.Helper()
	path, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just not installed on PATH, skipping")
	}
	return tools.NewJust(path)
}

func writeJustfile(t *testing.T, dir, name, body string) *project.JustfileInfo {
	t.Helper()
	path := filepath.Join(dir, name+".just")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write justfile: %v", err)
	}
	return &project.JustfileInfo{Name: name, Path: path}
}

func TestBuildJustArgs(t *testing.T) {
	just := ensureJust(t)
	ctx := context.Background()
	tmp := t.TempDir()

	t.Run("prepends filename when matching recipe exists", func(t *testing.T) {
		jf := writeJustfile(t, tmp, "console", "console *args:\n\t@echo {{args}}\n")
		got := buildJustArgs(ctx, just, jf, []string{"cache:clear"})
		want := []string{"console", "cache:clear"}
		if !equalStrings(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("prepends filename with no args so recipe still runs", func(t *testing.T) {
		jf := writeJustfile(t, tmp, "build", "build *args:\n\t@echo build {{args}}\n")
		got := buildJustArgs(ctx, just, jf, nil)
		want := []string{"build"}
		if !equalStrings(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("passes args through when no matching recipe", func(t *testing.T) {
		jf := writeJustfile(t, tmp, "setup", "default:\n\t@echo hello\ninstall:\n\t@echo install\n")
		got := buildJustArgs(ctx, just, jf, []string{"install"})
		want := []string{"install"}
		if !equalStrings(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("passes args through when justfile has syntax error", func(t *testing.T) {
		jf := writeJustfile(t, tmp, "broken", "this is not valid just syntax :::\n")
		got := buildJustArgs(ctx, just, jf, []string{"foo", "bar"})
		want := []string{"foo", "bar"}
		if !equalStrings(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
