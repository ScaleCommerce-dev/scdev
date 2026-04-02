//go:build integration

package create

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyLocal_Integration(t *testing.T) {
	// Create a realistic template structure
	srcDir := t.TempDir()

	// .scdev/config.yaml
	os.MkdirAll(filepath.Join(srcDir, ".scdev", "commands"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".scdev", "config.yaml"), []byte("version: 1\nname: ${PROJECTDIR}\n"), 0644)
	os.WriteFile(filepath.Join(srcDir, ".scdev", "commands", "setup.just"), []byte("default:\n    echo setup\n"), 0644)

	// App files
	os.WriteFile(filepath.Join(srcDir, "app.js"), []byte("console.log('hello');\n"), 0644)
	os.WriteFile(filepath.Join(srcDir, "package.json"), []byte("{}\n"), 0644)
	os.WriteFile(filepath.Join(srcDir, ".gitignore"), []byte("node_modules/\n"), 0644)

	// .git directory (should be excluded)
	os.MkdirAll(filepath.Join(srcDir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0644)

	// Copy to destination
	dstDir := t.TempDir()
	if err := CopyLocal(srcDir, dstDir); err != nil {
		t.Fatalf("CopyLocal failed: %v", err)
	}

	// Verify all expected files exist
	expectedFiles := []string{
		".scdev/config.yaml",
		".scdev/commands/setup.just",
		"app.js",
		"package.json",
		".gitignore",
	}
	for _, f := range expectedFiles {
		if _, err := os.Stat(filepath.Join(dstDir, f)); err != nil {
			t.Errorf("expected file %s not found", f)
		}
	}

	// Verify .git was excluded
	if _, err := os.Stat(filepath.Join(dstDir, ".git")); !os.IsNotExist(err) {
		t.Error(".git directory should not have been copied")
	}

	// Verify content integrity
	data, _ := os.ReadFile(filepath.Join(dstDir, ".scdev", "config.yaml"))
	if string(data) != "version: 1\nname: ${PROJECTDIR}\n" {
		t.Errorf("config content mismatch: %q", string(data))
	}
}

func TestCreateFromLocalTemplate_Integration(t *testing.T) {
	// Create a template
	templateDir := t.TempDir()
	os.MkdirAll(filepath.Join(templateDir, ".scdev", "commands"), 0755)
	os.WriteFile(filepath.Join(templateDir, ".scdev", "config.yaml"), []byte(`version: 1
name: ${PROJECTDIR}
services:
  app:
    image: alpine:latest
    command: sleep infinity
`), 0644)
	os.WriteFile(filepath.Join(templateDir, ".scdev", "commands", "setup.just"), []byte("default:\n    echo setup\n"), 0644)

	// Create project from template
	targetDir := filepath.Join(t.TempDir(), "my-project")

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}

	if err := CopyLocal(templateDir, targetDir); err != nil {
		t.Fatalf("CopyLocal failed: %v", err)
	}

	// Verify the project is a valid scdev project
	configPath := filepath.Join(targetDir, ".scdev", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatal("config.yaml not found in created project")
	}

	justfilePath := filepath.Join(targetDir, ".scdev", "commands", "setup.just")
	if _, err := os.Stat(justfilePath); err != nil {
		t.Fatal("setup.just not found in created project")
	}
}

func TestValidateName_EdgeCases(t *testing.T) {
	// Single char
	if err := ValidateName("a"); err != nil {
		t.Errorf("single char should be valid: %v", err)
	}

	// Two chars
	if err := ValidateName("ab"); err != nil {
		t.Errorf("two chars should be valid: %v", err)
	}

	// With numbers
	if err := ValidateName("app-2"); err != nil {
		t.Errorf("name with numbers should be valid: %v", err)
	}

	// Consecutive hyphens (valid in DNS)
	if err := ValidateName("my--app"); err != nil {
		t.Errorf("consecutive hyphens should be valid: %v", err)
	}

	// Just hyphens
	if err := ValidateName("-"); err == nil {
		t.Error("single hyphen should be invalid")
	}

	// Starts with number
	if err := ValidateName("1app"); err != nil {
		t.Errorf("starting with number should be valid: %v", err)
	}
}
