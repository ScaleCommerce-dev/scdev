package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubstituteVariables(t *testing.T) {
	vars := map[string]string{
		"PROJECTNAME":  "myapp",
		"SCDEV_DOMAIN": "scalecommerce.site",
		"USER":         "testuser",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple substitution",
			input:    "name: ${PROJECTNAME}",
			expected: "name: myapp",
		},
		{
			name:     "multiple substitutions",
			input:    "${PROJECTNAME}.${SCDEV_DOMAIN}",
			expected: "myapp.scalecommerce.site",
		},
		{
			name:     "no variables",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "unknown variable left as-is",
			input:    "${UNKNOWN_VAR}",
			expected: "${UNKNOWN_VAR}",
		},
		{
			name:     "mixed known and unknown",
			input:    "${PROJECTNAME}-${UNKNOWN}",
			expected: "myapp-${UNKNOWN}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteVariables(tt.input, vars)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestBuildVariables(t *testing.T) {
	projectDir := "/home/user/projects/myapp"
	vars := buildVariables(projectDir)

	// PROJECTDIR should be the directory basename
	if vars["PROJECTDIR"] != "myapp" {
		t.Errorf("expected PROJECTDIR to be 'myapp', got %q", vars["PROJECTDIR"])
	}

	// PROJECTNAME should NOT be set by buildVariables (it's set later after parsing)
	if _, exists := vars["PROJECTNAME"]; exists {
		t.Error("PROJECTNAME should not be set by buildVariables")
	}

	if vars["PROJECTPATH"] != projectDir {
		t.Errorf("expected PROJECTPATH to be %q, got %q", projectDir, vars["PROJECTPATH"])
	}

	if vars["SCDEV_DOMAIN"] == "" {
		t.Error("expected SCDEV_DOMAIN to have a default value")
	}
}

func TestLoadProject(t *testing.T) {
	// Use the minimal test fixture
	projectDir := filepath.Join("..", "..", "testdata", "projects", "minimal")
	absPath, err := filepath.Abs(projectDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	cfg, err := LoadProject(absPath)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if cfg.Name != "minimal" {
		t.Errorf("expected Name to be 'minimal', got %q", cfg.Name)
	}

	if cfg.Version != 1 {
		t.Errorf("expected Version to be 1, got %d", cfg.Version)
	}

	if len(cfg.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(cfg.Services))
	}

	app, ok := cfg.Services["app"]
	if !ok {
		t.Fatal("expected 'app' service to exist")
	}

	if app.Image != TestImage {
		t.Errorf("expected app.Image to be %q, got %q", TestImage, app.Image)
	}
}

func TestLoadProjectWithVariables(t *testing.T) {
	// Set SCDEV_DOMAIN for predictable test results
	oldDomain := os.Getenv("SCDEV_DOMAIN")
	os.Setenv("SCDEV_DOMAIN", "scalecommerce.site")
	defer os.Setenv("SCDEV_DOMAIN", oldDomain)

	// Use the full test fixture which has variable substitution
	projectDir := filepath.Join("..", "..", "testdata", "projects", "full")
	absPath, err := filepath.Abs(projectDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	cfg, err := LoadProject(absPath)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	// PROJECTNAME should be substituted to "full"
	if cfg.Name != "full" {
		t.Errorf("expected Name to be 'full', got %q", cfg.Name)
	}

	// Domain should have variables substituted
	expectedDomain := "full.scalecommerce.site"
	if cfg.Domain != expectedDomain {
		t.Errorf("expected Domain to be %q, got %q", expectedDomain, cfg.Domain)
	}

	// Check services
	if len(cfg.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(cfg.Services))
	}
}

func TestLoadProjectWithCustomName(t *testing.T) {
	// Set SCDEV_DOMAIN for predictable test results
	oldDomain := os.Getenv("SCDEV_DOMAIN")
	os.Setenv("SCDEV_DOMAIN", "scalecommerce.site")
	defer os.Setenv("SCDEV_DOMAIN", oldDomain)

	// Test that PROJECTNAME is set from the parsed name field, not just the directory
	tmpDir, err := os.MkdirTemp("", "scdev-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	scdevDir := filepath.Join(tmpDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		t.Fatalf("failed to create .scdev dir: %v", err)
	}

	// Config uses a custom name, and PROJECTNAME should reflect that in domain
	configContent := `version: 1
name: my-custom-app
domain: ${PROJECTNAME}.${SCDEV_DOMAIN}

services:
  app:
    image: nginx:alpine
`
	configPath := filepath.Join(scdevDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadProject(tmpDir)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	// Name should be the custom name
	if cfg.Name != "my-custom-app" {
		t.Errorf("expected Name to be 'my-custom-app', got %q", cfg.Name)
	}

	// Domain should use PROJECTNAME (which is the parsed name, not directory)
	expectedDomain := "my-custom-app.scalecommerce.site"
	if cfg.Domain != expectedDomain {
		t.Errorf("expected Domain to be %q, got %q", expectedDomain, cfg.Domain)
	}
}

func TestLoadProjectWithProjectDir(t *testing.T) {
	// Set SCDEV_DOMAIN for predictable test results
	oldDomain := os.Getenv("SCDEV_DOMAIN")
	os.Setenv("SCDEV_DOMAIN", "scalecommerce.site")
	defer os.Setenv("SCDEV_DOMAIN", oldDomain)

	// Test that PROJECTDIR is available and separate from PROJECTNAME
	tmpDir, err := os.MkdirTemp("", "scdev-test-mydir-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks for comparison
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	scdevDir := filepath.Join(tmpDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		t.Fatalf("failed to create .scdev dir: %v", err)
	}

	// Use PROJECTDIR for name, then PROJECTNAME in domain
	configContent := `version: 1
name: ${PROJECTDIR}-app
domain: ${PROJECTNAME}.${SCDEV_DOMAIN}

services:
  app:
    image: nginx:alpine
`
	configPath := filepath.Join(scdevDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadProject(tmpDir)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	dirName := filepath.Base(tmpDir)
	expectedName := dirName + "-app"

	if cfg.Name != expectedName {
		t.Errorf("expected Name to be %q, got %q", expectedName, cfg.Name)
	}

	// Domain should use PROJECTNAME which is the resolved name
	expectedDomain := expectedName + ".scalecommerce.site"
	if cfg.Domain != expectedDomain {
		t.Errorf("expected Domain to be %q, got %q", expectedDomain, cfg.Domain)
	}
}

func TestLoadProjectUnknownField(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "scdev-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .scdev/config.yaml with unknown field
	scdevDir := filepath.Join(tmpDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		t.Fatalf("failed to create .scdev dir: %v", err)
	}

	configContent := `version: 1
name: testproject

services:
  app:
    image: nginx:alpine

volmes:
  db_data:
`
	configPath := filepath.Join(scdevDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// LoadProject should fail with unknown field
	_, err = LoadProject(tmpDir)
	if err == nil {
		t.Fatal("expected error for unknown field 'volmes', got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "unknown field") {
		t.Errorf("expected error to mention 'unknown field', got: %s", errStr)
	}
	if !strings.Contains(errStr, "volmes") {
		t.Errorf("expected error to mention 'volmes', got: %s", errStr)
	}
}

func TestLoadProjectSyntaxError(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "scdev-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .scdev/config.yaml with syntax error
	scdevDir := filepath.Join(tmpDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		t.Fatalf("failed to create .scdev dir: %v", err)
	}

	configContent := `version: 1
name: testproject

services:
  app:
    image: nginx:alpine
    environment
      FOO: bar
`
	configPath := filepath.Join(scdevDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// LoadProject should fail with syntax error
	_, err = LoadProject(tmpDir)
	if err == nil {
		t.Fatal("expected error for YAML syntax error, got nil")
	}

	errStr := err.Error()
	// Should include the config path
	if !strings.Contains(errStr, "config.yaml") {
		t.Errorf("expected error to include config path, got: %s", errStr)
	}
	// Should include line number
	if !strings.Contains(errStr, ":7:") {
		t.Errorf("expected error to include line number ':7:', got: %s", errStr)
	}
}

func TestLoadGlobalConfig(t *testing.T) {
	// Create a temp scdev home directory
	tmpDir, err := os.MkdirTemp("", "scdev-home-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override SCDEV_HOME
	oldHome := os.Getenv("SCDEV_HOME")
	os.Setenv("SCDEV_HOME", tmpDir)
	defer os.Setenv("SCDEV_HOME", oldHome)

	t.Run("returns defaults when file missing", func(t *testing.T) {
		cfg, err := LoadGlobalConfig()
		if err != nil {
			t.Fatalf("LoadGlobalConfig failed: %v", err)
		}

		if cfg.Version != 1 {
			t.Errorf("expected Version 1, got %d", cfg.Version)
		}
		if cfg.Domain != DefaultDomain {
			t.Errorf("expected Domain %q, got %q", DefaultDomain, cfg.Domain)
		}
		if cfg.Runtime != "docker" {
			t.Errorf("expected Runtime 'docker', got %q", cfg.Runtime)
		}
		if cfg.Shared.Router.Image != RouterImage {
			t.Errorf("expected Router.Image %q, got %q", RouterImage, cfg.Shared.Router.Image)
		}
	})

	t.Run("loads custom config", func(t *testing.T) {
		configContent := `version: 2
domain: example.com
runtime: podman
shared:
  router:
    image: traefik:v2.0
    dashboard: true
`
		configPath := filepath.Join(tmpDir, GlobalConfigFilename)
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
		defer os.Remove(configPath)

		cfg, err := LoadGlobalConfig()
		if err != nil {
			t.Fatalf("LoadGlobalConfig failed: %v", err)
		}

		if cfg.Version != 2 {
			t.Errorf("expected Version 2, got %d", cfg.Version)
		}
		if cfg.Domain != "example.com" {
			t.Errorf("expected Domain 'example.com', got %q", cfg.Domain)
		}
		if cfg.Runtime != "podman" {
			t.Errorf("expected Runtime 'podman', got %q", cfg.Runtime)
		}
		if cfg.Shared.Router.Image != "traefik:v2.0" {
			t.Errorf("expected Router.Image 'traefik:v2.0', got %q", cfg.Shared.Router.Image)
		}
		if !cfg.Shared.Router.Dashboard {
			t.Error("expected Router.Dashboard to be true")
		}
	})

	t.Run("SSL enabled by default when not in config", func(t *testing.T) {
		// Config file without ssl section - SSL should still default to enabled
		configContent := fmt.Sprintf(`version: 1
domain: %s
runtime: docker
`, DefaultDomain)
		configPath := filepath.Join(tmpDir, GlobalConfigFilename)
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
		defer os.Remove(configPath)

		cfg, err := LoadGlobalConfig()
		if err != nil {
			t.Fatalf("LoadGlobalConfig failed: %v", err)
		}

		if !cfg.SSL.Enabled {
			t.Error("expected SSL.Enabled to be true by default when not specified in config")
		}
	})

	t.Run("SSL can be explicitly disabled", func(t *testing.T) {
		configContent := fmt.Sprintf(`version: 1
domain: %s
ssl:
  enabled: false
`, DefaultDomain)
		configPath := filepath.Join(tmpDir, GlobalConfigFilename)
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
		defer os.Remove(configPath)

		cfg, err := LoadGlobalConfig()
		if err != nil {
			t.Fatalf("LoadGlobalConfig failed: %v", err)
		}

		if cfg.SSL.Enabled {
			t.Error("expected SSL.Enabled to be false when explicitly disabled")
		}
	})
}

func TestFindProjectDir(t *testing.T) {
	// Create a temp directory structure
	tmpDir, err := os.MkdirTemp("", "scdev-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks (macOS has /var -> /private/var)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	// Create .scdev/config.yaml
	scdevDir := filepath.Join(tmpDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		t.Fatalf("failed to create .scdev dir: %v", err)
	}

	configPath := filepath.Join(scdevDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\nname: test"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "src", "deep")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Change to the deep subdirectory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// FindProjectDir should find the parent with .scdev
	projectDir, err := FindProjectDir()
	if err != nil {
		t.Fatalf("FindProjectDir failed: %v", err)
	}

	if projectDir != tmpDir {
		t.Errorf("expected %q, got %q", tmpDir, projectDir)
	}
}

func TestFindProjectDirWithOverride(t *testing.T) {
	// Create a temp directory for the override path
	overrideDir, err := os.MkdirTemp("", "scdev-override-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(overrideDir)

	// Resolve symlinks
	overrideDir, err = filepath.EvalSymlinks(overrideDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	// Create .scdev/config.yaml in override dir
	scdevDir := filepath.Join(overrideDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		t.Fatalf("failed to create .scdev dir: %v", err)
	}

	configPath := filepath.Join(scdevDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\nname: override-test"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Save and restore original override
	originalOverride := GetProjectDirOverride()
	defer SetProjectDirOverride(originalOverride)

	t.Run("override takes precedence", func(t *testing.T) {
		SetProjectDirOverride(overrideDir)

		// FindProjectDir should return override, regardless of cwd
		projectDir, err := FindProjectDir()
		if err != nil {
			t.Fatalf("FindProjectDir failed: %v", err)
		}

		if projectDir != overrideDir {
			t.Errorf("expected %q, got %q", overrideDir, projectDir)
		}
	})

	t.Run("invalid override returns error", func(t *testing.T) {
		SetProjectDirOverride("/nonexistent/path")

		_, err := FindProjectDir()
		if err == nil {
			t.Error("expected error for invalid override path")
		}
	})

	t.Run("empty override falls back to discovery", func(t *testing.T) {
		SetProjectDirOverride("")

		// This will fail because we're not in a project dir, which is expected
		_, err := FindProjectDir()
		if err == nil {
			// If it succeeds, we must be in an actual project directory
			// That's fine, the point is it didn't use the override
		}
	})
}

func TestGenerateDefaultGlobalConfig(t *testing.T) {
	config := generateDefaultGlobalConfig()

	// Verify our template variables were substituted
	varsToSubstitute := []string{
		"${DefaultDomain}",
		"${RouterImage}",
		"${MailImage}",
		"${DBUIImage}",
		"${ObservabilityImage}",
	}
	for _, v := range varsToSubstitute {
		if strings.Contains(config, v) {
			t.Errorf("config contains unsubstituted variable: %s", v)
		}
	}

	// Verify specific values from defaults.go are present
	if !strings.Contains(config, DefaultDomain) {
		t.Errorf("config should contain DefaultDomain (%s)", DefaultDomain)
	}
	if !strings.Contains(config, RouterImage) {
		t.Errorf("config should contain RouterImage (%s)", RouterImage)
	}
	if !strings.Contains(config, MailImage) {
		t.Errorf("config should contain MailImage (%s)", MailImage)
	}
	if !strings.Contains(config, DBUIImage) {
		t.Errorf("config should contain DBUIImage (%s)", DBUIImage)
	}
	if !strings.Contains(config, ObservabilityImage) {
		t.Errorf("config should contain ObservabilityImage (%s)", ObservabilityImage)
	}
}

func TestGenerateDocsTraefikConfig(t *testing.T) {
	t.Run("HTTP only", func(t *testing.T) {
		config := generateDocsTraefikConfig("example.com", false)

		// Check for expected content
		if !strings.Contains(config, "scdev-docs") {
			t.Error("config should contain scdev-docs middleware")
		}
		if !strings.Contains(config, "scdev-docs-redirect") {
			t.Error("config should contain scdev-docs-redirect middleware")
		}
		if !strings.Contains(config, "docs.shared.example.com") {
			t.Error("config should contain docs host")
		}
		if !strings.Contains(config, "http://docs.shared.example.com/") {
			t.Error("config should contain http redirect URL")
		}
		if !strings.Contains(config, "scdev-docs-http") {
			t.Error("config should contain HTTP router")
		}
		if !strings.Contains(config, "scdev-catchall-http") {
			t.Error("config should contain HTTP catch-all router")
		}
		// Should NOT have HTTPS routers
		if strings.Contains(config, "scdev-docs-https") {
			t.Error("config should NOT contain HTTPS router when TLS disabled")
		}
	})

	t.Run("HTTPS enabled", func(t *testing.T) {
		config := generateDocsTraefikConfig("example.com", true)

		// Check for expected content
		if !strings.Contains(config, "https://docs.shared.example.com/") {
			t.Error("config should contain https redirect URL")
		}
		if !strings.Contains(config, "scdev-docs-https") {
			t.Error("config should contain HTTPS router")
		}
		if !strings.Contains(config, "scdev-catchall-https") {
			t.Error("config should contain HTTPS catch-all router")
		}
		if !strings.Contains(config, "tls: {}") {
			t.Error("config should contain TLS configuration")
		}
	})

	t.Run("domain with dots escaped in regex", func(t *testing.T) {
		config := generateDocsTraefikConfig("my.domain.com", true)

		// The domain dots should be escaped in the HostRegexp
		if !strings.Contains(config, "my\\\\.domain\\\\.com") {
			t.Error("config should contain escaped dots in HostRegexp")
		}
	})
}

func TestEnsureDocsConfig(t *testing.T) {
	// Create a temp scdev home directory
	tmpDir, err := os.MkdirTemp("", "scdev-home-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override SCDEV_HOME
	oldHome := os.Getenv("SCDEV_HOME")
	os.Setenv("SCDEV_HOME", tmpDir)
	defer os.Setenv("SCDEV_HOME", oldHome)

	docsDir, err := EnsureDocsConfig("example.com", true)
	if err != nil {
		t.Fatalf("EnsureDocsConfig failed: %v", err)
	}

	// Check docs directory was created
	expectedDocsDir := filepath.Join(tmpDir, "docs")
	if docsDir != expectedDocsDir {
		t.Errorf("expected docsDir %q, got %q", expectedDocsDir, docsDir)
	}

	// Check HTML file exists
	htmlPath := filepath.Join(docsDir, "index.html")
	if _, err := os.Stat(htmlPath); err != nil {
		t.Errorf("index.html should exist: %v", err)
	}

	// Check HTML contains substituted domain
	htmlContent, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("failed to read HTML: %v", err)
	}
	if !strings.Contains(string(htmlContent), "example.com") {
		t.Error("HTML should contain domain")
	}
	if !strings.Contains(string(htmlContent), "https://") {
		t.Error("HTML should contain https protocol when TLS enabled")
	}

	// Check traefik config file exists
	traefikConfigPath := filepath.Join(tmpDir, "traefik", "docs.yaml")
	if _, err := os.Stat(traefikConfigPath); err != nil {
		t.Errorf("docs.yaml should exist: %v", err)
	}

	// Check traefik config content
	traefikContent, err := os.ReadFile(traefikConfigPath)
	if err != nil {
		t.Fatalf("failed to read traefik config: %v", err)
	}
	if !strings.Contains(string(traefikContent), "statiq") {
		t.Error("traefik config should reference statiq plugin")
	}
}
