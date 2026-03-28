package tools

import (
	"testing"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
)

func TestMutagenTool(t *testing.T) {
	tool := MutagenTool()

	if tool.Name != "mutagen" {
		t.Errorf("expected Name 'mutagen', got %q", tool.Name)
	}

	if tool.Version != config.MutagenVersion {
		t.Errorf("expected Version %q, got %q", config.MutagenVersion, tool.Version)
	}

	if tool.BinaryName != "mutagen" {
		t.Errorf("expected BinaryName 'mutagen', got %q", tool.BinaryName)
	}

	if tool.URLTemplate != config.MutagenURLTemplate {
		t.Errorf("expected URLTemplate from config, got %q", tool.URLTemplate)
	}

	if tool.ArchiveType != "tar.gz" {
		t.Errorf("expected ArchiveType 'tar.gz', got %q", tool.ArchiveType)
	}

	if tool.URLBuilder == nil {
		t.Error("expected URLBuilder to be set")
	}

	// Verify ExtraFiles includes the agents bundle
	if len(tool.ExtraFiles) != 1 {
		t.Errorf("expected 1 extra file, got %d", len(tool.ExtraFiles))
	}
	if tool.ExtraFiles[0] != "mutagen-agents.tar.gz" {
		t.Errorf("expected ExtraFiles[0] 'mutagen-agents.tar.gz', got %q", tool.ExtraFiles[0])
	}
}

func TestMutagenURLBuilder(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		goos     string
		goarch   string
		expected string
	}{
		{
			name:     "darwin amd64",
			version:  "0.18.1",
			goos:     "darwin",
			goarch:   "amd64",
			expected: "https://github.com/mutagen-io/mutagen/releases/download/v0.18.1/mutagen_darwin_amd64_v0.18.1.tar.gz",
		},
		{
			name:     "darwin arm64",
			version:  "0.18.1",
			goos:     "darwin",
			goarch:   "arm64",
			expected: "https://github.com/mutagen-io/mutagen/releases/download/v0.18.1/mutagen_darwin_arm64_v0.18.1.tar.gz",
		},
		{
			name:     "linux amd64",
			version:  "0.18.1",
			goos:     "linux",
			goarch:   "amd64",
			expected: "https://github.com/mutagen-io/mutagen/releases/download/v0.18.1/mutagen_linux_amd64_v0.18.1.tar.gz",
		},
		{
			name:     "linux arm64",
			version:  "0.18.1",
			goos:     "linux",
			goarch:   "arm64",
			expected: "https://github.com/mutagen-io/mutagen/releases/download/v0.18.1/mutagen_linux_arm64_v0.18.1.tar.gz",
		},
		{
			name:     "different version",
			version:  "0.19.0",
			goos:     "darwin",
			goarch:   "arm64",
			expected: "https://github.com/mutagen-io/mutagen/releases/download/v0.19.0/mutagen_darwin_arm64_v0.19.0.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := mutagenURLBuilder(config.MutagenURLTemplate, tt.version, tt.goos, tt.goarch)
			if url != tt.expected {
				t.Errorf("mutagenURLBuilder() = %q, want %q", url, tt.expected)
			}
		})
	}
}

func TestBuildDownloadURL_Mutagen(t *testing.T) {
	tool := MutagenTool()
	url := buildDownloadURL(tool)

	// URL should contain version
	if !contains(url, config.MutagenVersion) {
		t.Errorf("URL should contain version %s, got: %s", config.MutagenVersion, url)
	}

	// URL should end with .tar.gz
	if !contains(url, ".tar.gz") {
		t.Errorf("URL should contain .tar.gz, got: %s", url)
	}

	// URL should contain mutagen
	if !contains(url, "mutagen") {
		t.Errorf("URL should contain 'mutagen', got: %s", url)
	}

	// URL should point to GitHub
	if !contains(url, "github.com/mutagen-io/mutagen") {
		t.Errorf("URL should point to mutagen-io/mutagen repo, got: %s", url)
	}
}
