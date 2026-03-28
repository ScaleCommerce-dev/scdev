package tools

import (
	"runtime"
	"testing"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
)

func TestFindInPath(t *testing.T) {
	// Test with a command that should exist on all systems
	t.Run("finds existing command", func(t *testing.T) {
		// 'ls' exists on macOS/Linux, 'cmd' on Windows
		var cmdName string
		if runtime.GOOS == "windows" {
			cmdName = "cmd"
		} else {
			cmdName = "ls"
		}

		path, found := FindInPath(cmdName)
		if !found {
			t.Errorf("expected to find %s in PATH", cmdName)
		}
		if path == "" {
			t.Error("expected non-empty path")
		}
	})

	t.Run("does not find nonexistent command", func(t *testing.T) {
		path, found := FindInPath("scdev-nonexistent-command-12345")
		if found {
			t.Error("expected not to find nonexistent command")
		}
		if path != "" {
			t.Error("expected empty path for nonexistent command")
		}
	})
}

func TestBuildDownloadURL(t *testing.T) {
	tool := ToolInfo{
		Name:        "mkcert",
		Version:     "v1.4.4",
		URLTemplate: "https://github.com/FiloSottile/mkcert/releases/download/%s/mkcert-%s-%s-%s",
		BinaryName:  "mkcert",
	}

	url := buildDownloadURL(tool)

	// URL should contain version, OS, and arch
	expectedOS := runtime.GOOS
	expectedArch := runtime.GOARCH

	if url == "" {
		t.Error("expected non-empty URL")
	}

	// Check that URL contains expected components
	if !contains(url, "v1.4.4") {
		t.Errorf("URL should contain version, got: %s", url)
	}
	if !contains(url, expectedOS) {
		t.Errorf("URL should contain OS %s, got: %s", expectedOS, url)
	}
	if !contains(url, expectedArch) {
		t.Errorf("URL should contain arch %s, got: %s", expectedArch, url)
	}
}

func TestMkcertTool(t *testing.T) {
	tool := MkcertTool()

	if tool.Name != "mkcert" {
		t.Errorf("expected Name 'mkcert', got %q", tool.Name)
	}

	if tool.Version != config.MkcertVersion {
		t.Errorf("expected Version %q, got %q", config.MkcertVersion, tool.Version)
	}

	if tool.BinaryName != "mkcert" {
		t.Errorf("expected BinaryName 'mkcert', got %q", tool.BinaryName)
	}

	if tool.URLTemplate != config.MkcertURLTemplate {
		t.Errorf("expected URLTemplate from config, got %q", tool.URLTemplate)
	}
}

func TestGetArch(t *testing.T) {
	arch := GetArch()

	// Should return a non-empty string
	if arch == "" {
		t.Error("expected non-empty architecture string")
	}

	// Should be one of the known architectures
	validArchs := map[string]bool{
		"amd64": true,
		"arm64": true,
		"386":   true,
		"arm":   true,
	}

	if !validArchs[arch] {
		t.Logf("unexpected architecture: %s (this may be valid for your platform)", arch)
	}
}

func TestGetOS(t *testing.T) {
	os := GetOS()

	// Should return a non-empty string
	if os == "" {
		t.Error("expected non-empty OS string")
	}

	// Should match runtime.GOOS
	if os != runtime.GOOS {
		t.Errorf("expected %q, got %q", runtime.GOOS, os)
	}
}

func TestJustArch(t *testing.T) {
	tests := []struct {
		goarch   string
		expected string
	}{
		{"amd64", "x86_64"},
		{"arm64", "aarch64"},
		{"386", "386"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.goarch, func(t *testing.T) {
			result := JustArch(tt.goarch)
			if result != tt.expected {
				t.Errorf("JustArch(%q) = %q, want %q", tt.goarch, result, tt.expected)
			}
		})
	}
}

func TestJustOS(t *testing.T) {
	tests := []struct {
		goos     string
		expected string
	}{
		{"darwin", "apple-darwin"},
		{"linux", "unknown-linux-musl"},
		{"windows", "windows"},
		{"freebsd", "freebsd"},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			result := JustOS(tt.goos)
			if result != tt.expected {
				t.Errorf("JustOS(%q) = %q, want %q", tt.goos, result, tt.expected)
			}
		})
	}
}

func TestJustTool(t *testing.T) {
	tool := JustTool()

	if tool.Name != "just" {
		t.Errorf("expected Name 'just', got %q", tool.Name)
	}

	if tool.Version != config.JustVersion {
		t.Errorf("expected Version %q, got %q", config.JustVersion, tool.Version)
	}

	if tool.BinaryName != "just" {
		t.Errorf("expected BinaryName 'just', got %q", tool.BinaryName)
	}

	if tool.ArchiveType != "tar.gz" {
		t.Errorf("expected ArchiveType 'tar.gz', got %q", tool.ArchiveType)
	}

	if tool.URLBuilder == nil {
		t.Error("expected URLBuilder to be set")
	}
}

func TestBuildDownloadURLWithCustomBuilder(t *testing.T) {
	tool := JustTool()
	url := buildDownloadURL(tool)

	// URL should contain version
	if !contains(url, config.JustVersion) {
		t.Errorf("URL should contain version %s, got: %s", config.JustVersion, url)
	}

	// URL should end with .tar.gz
	if !contains(url, ".tar.gz") {
		t.Errorf("URL should contain .tar.gz, got: %s", url)
	}

	// URL should contain just-specific arch/os naming
	expectedArch := JustArch(runtime.GOARCH)
	expectedOS := JustOS(runtime.GOOS)

	if !contains(url, expectedArch) {
		t.Errorf("URL should contain arch %s, got: %s", expectedArch, url)
	}
	if !contains(url, expectedOS) {
		t.Errorf("URL should contain OS %s, got: %s", expectedOS, url)
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
