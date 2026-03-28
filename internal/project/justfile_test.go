package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetJustfileFromDir(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "scdev-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .scdev/commands directory
	commandsDir := filepath.Join(tmpDir, ".scdev", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}

	// Create a test justfile
	testJustfile := filepath.Join(commandsDir, "setup.just")
	if err := os.WriteFile(testJustfile, []byte("default:\n\t@echo hello"), 0644); err != nil {
		t.Fatalf("failed to write test justfile: %v", err)
	}

	t.Run("finds existing justfile", func(t *testing.T) {
		jf, err := GetJustfileFromDir(tmpDir, "setup")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if jf == nil {
			t.Fatal("expected to find justfile, got nil")
		}
		if jf.Name != "setup" {
			t.Errorf("expected Name 'setup', got %q", jf.Name)
		}
		if jf.Path != testJustfile {
			t.Errorf("expected Path %q, got %q", testJustfile, jf.Path)
		}
	})

	t.Run("returns nil for nonexistent justfile", func(t *testing.T) {
		jf, err := GetJustfileFromDir(tmpDir, "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if jf != nil {
			t.Errorf("expected nil for nonexistent justfile, got %+v", jf)
		}
	})

	t.Run("returns nil for nonexistent project dir", func(t *testing.T) {
		jf, err := GetJustfileFromDir("/nonexistent/path", "setup")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if jf != nil {
			t.Errorf("expected nil for nonexistent dir, got %+v", jf)
		}
	})
}

func TestDiscoverJustfiles(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "scdev-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .scdev/commands directory
	commandsDir := filepath.Join(tmpDir, ".scdev", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}

	// Create test justfiles
	justfiles := []string{"setup.just", "test.just", "build.just"}
	for _, name := range justfiles {
		path := filepath.Join(commandsDir, name)
		if err := os.WriteFile(path, []byte("default:\n\t@echo "+name), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	// Create a non-just file that should be ignored
	otherFile := filepath.Join(commandsDir, "README.md")
	if err := os.WriteFile(otherFile, []byte("# Commands"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	// Create a subdirectory that should be ignored
	subDir := filepath.Join(commandsDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Create a minimal project config
	configPath := filepath.Join(tmpDir, ".scdev", "config.yaml")
	configContent := `version: 1
name: test-project
services:
  app:
    image: alpine:latest
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Load project
	proj, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to load project: %v", err)
	}

	// Discover justfiles
	found, err := proj.DiscoverJustfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(found) != 3 {
		t.Errorf("expected 3 justfiles, found %d", len(found))
	}

	// Check that all expected justfiles are found
	foundNames := make(map[string]bool)
	for _, jf := range found {
		foundNames[jf.Name] = true
	}

	for _, expected := range []string{"setup", "test", "build"} {
		if !foundNames[expected] {
			t.Errorf("expected to find justfile %q", expected)
		}
	}
}

func TestDiscoverJustfilesNoCommandsDir(t *testing.T) {
	// Create temp directory without commands dir
	tmpDir, err := os.MkdirTemp("", "scdev-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create minimal .scdev directory with config only
	scdevDir := filepath.Join(tmpDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		t.Fatalf("failed to create .scdev dir: %v", err)
	}

	configPath := filepath.Join(scdevDir, "config.yaml")
	configContent := `version: 1
name: test-project
services:
  app:
    image: alpine:latest
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Load project
	proj, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to load project: %v", err)
	}

	// Discover justfiles - should return empty, not error
	found, err := proj.DiscoverJustfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(found) != 0 {
		t.Errorf("expected 0 justfiles, found %d", len(found))
	}
}

func TestBuildJustEnv(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "scdev-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .scdev directory
	scdevDir := filepath.Join(tmpDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		t.Fatalf("failed to create .scdev dir: %v", err)
	}

	configPath := filepath.Join(scdevDir, "config.yaml")
	configContent := `version: 1
name: my-project
environment:
  APP_ENV: development
  DEBUG: "true"
services:
  app:
    image: alpine:latest
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Load project
	proj, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to load project: %v", err)
	}

	// Build environment
	env := proj.BuildJustEnv()

	// Check required variables
	if env["PROJECTNAME"] != "my-project" {
		t.Errorf("expected PROJECTNAME 'my-project', got %q", env["PROJECTNAME"])
	}

	if env["PROJECTPATH"] != tmpDir {
		t.Errorf("expected PROJECTPATH %q, got %q", tmpDir, env["PROJECTPATH"])
	}

	if env["PROJECTDIR"] != filepath.Base(tmpDir) {
		t.Errorf("expected PROJECTDIR %q, got %q", filepath.Base(tmpDir), env["PROJECTDIR"])
	}

	// Check that SCDEV_DOMAIN and SCDEV_HOME are set (values depend on system)
	if env["SCDEV_DOMAIN"] == "" {
		t.Error("expected SCDEV_DOMAIN to be set")
	}

	if env["SCDEV_HOME"] == "" {
		t.Error("expected SCDEV_HOME to be set")
	}

	// Check project environment variables
	if env["APP_ENV"] != "development" {
		t.Errorf("expected APP_ENV 'development', got %q", env["APP_ENV"])
	}

	if env["DEBUG"] != "true" {
		t.Errorf("expected DEBUG 'true', got %q", env["DEBUG"])
	}

	// Check that PATH is inherited from current environment
	if env["PATH"] == "" {
		t.Error("expected PATH to be inherited from environment")
	}
}
