package create

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{
		"myapp",
		"my-app",
		"a",
		"a1",
		"my-express-app",
		"app123",
		"a-b-c",
	}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",
		"MyApp",
		"-myapp",
		"myapp-",
		"-",
		"my_app",
		"my app",
		"my.app",
		"ALLCAPS",
	}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", name)
		}
	}

	// Max length (63 chars)
	longValid := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0" // 63 chars
	if err := ValidateName(longValid); err != nil {
		t.Errorf("ValidateName(63 chars) = %v, want nil", err)
	}

	// Too long (64 chars)
	tooLong := longValid + "a"
	if err := ValidateName(tooLong); err == nil {
		t.Errorf("ValidateName(64 chars) = nil, want error")
	}
}

func TestResolveTemplate_Local(t *testing.T) {
	// Create a temp directory to use as a template
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		template string
		wantType string
		wantErr  bool
	}{
		{
			name:     "absolute path",
			template: tmpDir,
			wantType: "local",
		},
		{
			name:     "relative path with dot",
			template: ".",
			wantType: "local",
		},
		{
			name:     "relative path with dot-dot",
			template: "..",
			wantType: "local",
		},
		{
			name:     "nonexistent path",
			template: "/nonexistent/path/to/template",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := ResolveTemplate(tt.template, "", "")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if src.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", src.Type, tt.wantType)
			}
		})
	}
}

func TestResolveTemplate_GitHub(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		branch    string
		tag       string
		wantOwner string
		wantRepo  string
		wantRef   string
		wantErr   bool
	}{
		{
			name:      "full owner/repo",
			template:  "myorg/mytemplate",
			wantOwner: "myorg",
			wantRepo:  "mytemplate",
		},
		{
			name:      "bare name",
			template:  "express",
			wantOwner: "ScaleCommerce-DEV",
			wantRepo:  "scdev-template-express",
		},
		{
			name:      "bare name with branch",
			template:  "express",
			branch:    "develop",
			wantOwner: "ScaleCommerce-DEV",
			wantRepo:  "scdev-template-express",
			wantRef:   "develop",
		},
		{
			name:      "full repo with tag",
			template:  "myorg/myrepo",
			tag:       "v1.0",
			wantOwner: "myorg",
			wantRepo:  "myrepo",
			wantRef:   "v1.0",
		},
		{
			name:     "branch and tag both set",
			template: "express",
			branch:   "main",
			tag:      "v1.0",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := ResolveTemplate(tt.template, tt.branch, tt.tag)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if src.Type != "github" {
				t.Errorf("Type = %q, want %q", src.Type, "github")
			}
			if src.Owner != tt.wantOwner {
				t.Errorf("Owner = %q, want %q", src.Owner, tt.wantOwner)
			}
			if src.Repo != tt.wantRepo {
				t.Errorf("Repo = %q, want %q", src.Repo, tt.wantRepo)
			}
			if src.Ref != tt.wantRef {
				t.Errorf("Ref = %q, want %q", src.Ref, tt.wantRef)
			}
		})
	}
}

func TestResolveTemplate_BranchTagWithLocal(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := ResolveTemplate(tmpDir, "main", "")
	if err == nil {
		t.Fatal("expected error when using --branch with local template")
	}

	_, err = ResolveTemplate(tmpDir, "", "v1.0")
	if err == nil {
		t.Fatal("expected error when using --tag with local template")
	}
}

func TestCopyLocal(t *testing.T) {
	// Create source template structure
	srcDir := t.TempDir()

	// Create files and directories
	os.MkdirAll(filepath.Join(srcDir, ".scdev", "commands"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".scdev", "config.yaml"), []byte("version: 1"), 0644)
	os.WriteFile(filepath.Join(srcDir, ".scdev", "commands", "setup.just"), []byte("default:\n  echo hi"), 0644)
	os.WriteFile(filepath.Join(srcDir, "app.js"), []byte("console.log('hello')"), 0644)
	os.WriteFile(filepath.Join(srcDir, "package.json"), []byte("{}"), 0644)

	// Create a .git directory that should be excluded
	os.MkdirAll(filepath.Join(srcDir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(srcDir, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)

	// Copy to destination
	dstDir := t.TempDir()
	if err := CopyLocal(srcDir, dstDir); err != nil {
		t.Fatalf("CopyLocal() error: %v", err)
	}

	// Verify expected files exist
	expectedFiles := []string{
		".scdev/config.yaml",
		".scdev/commands/setup.just",
		"app.js",
		"package.json",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(dstDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s not found: %v", f, err)
		}
	}

	// Verify .git was excluded
	gitDir := filepath.Join(dstDir, ".git")
	if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
		t.Errorf(".git directory should not have been copied")
	}

	// Verify file contents
	data, err := os.ReadFile(filepath.Join(dstDir, ".scdev", "config.yaml"))
	if err != nil {
		t.Fatalf("failed to read copied config: %v", err)
	}
	if string(data) != "version: 1" {
		t.Errorf("config content = %q, want %q", string(data), "version: 1")
	}
}

func TestResolveTemplate_FileNotDir(t *testing.T) {
	// Create a temp file (not directory)
	tmpFile := filepath.Join(t.TempDir(), "notadir")
	os.WriteFile(tmpFile, []byte("test"), 0644)

	_, err := ResolveTemplate(tmpFile, "", "")
	if err == nil {
		t.Fatal("expected error when template is a file, not directory")
	}
}
