package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestJustRun_RecipeCwdMatchesWorkingDir locks down the regression that broke
// `zdev setup`: just chdirs to the justfile's parent dir when --justfile is
// passed, so just relying on exec.Cmd.Dir leaves recipes running from
// .zdev/commands/ instead of the project root the caller asked for. The
// recipe writes its cwd to a marker file; we assert the marker landed in
// the requested working dir, not in the justfile's parent.
func TestJustRun_RecipeCwdMatchesWorkingDir(t *testing.T) {
	if _, err := exec.LookPath("just"); err != nil {
		t.Skip("just not installed on PATH, skipping")
	}

	root := t.TempDir()
	justfileDir := filepath.Join(root, ".zdev", "commands")
	if err := os.MkdirAll(justfileDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	justfilePath := filepath.Join(justfileDir, "setup.just")
	body := "default:\n    pwd > cwd-marker.txt\n"
	if err := os.WriteFile(justfilePath, []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	just := NewJust("just")
	if err := just.Run(context.Background(), justfilePath, root, nil, nil); err != nil {
		t.Fatalf("just.Run: %v", err)
	}

	marker := filepath.Join(root, "cwd-marker.txt")
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("recipe did not write %s (so cwd != workingDir): %v", marker, err)
	}

	got := strings.TrimSpace(string(data))
	wantRoot, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantRoot {
		t.Errorf("recipe cwd = %q, want %q", gotResolved, wantRoot)
	}

	// Make sure the marker did NOT land in the justfile's directory.
	if _, err := os.Stat(filepath.Join(justfileDir, "cwd-marker.txt")); err == nil {
		t.Errorf("recipe wrote marker to justfile dir; cwd was not redirected")
	}
}
