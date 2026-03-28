package tools

import (
	"fmt"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
)

// MutagenTool returns the ToolInfo for mutagen
func MutagenTool() ToolInfo {
	return ToolInfo{
		Name:        "mutagen",
		Version:     config.MutagenVersion,
		URLTemplate: config.MutagenURLTemplate,
		BinaryName:  "mutagen",
		ArchiveType: "tar.gz",
		URLBuilder:  mutagenURLBuilder,
		ExtraFiles:  []string{"mutagen-agents.tar.gz"}, // Required for container sync
	}
}

// mutagenURLBuilder constructs the download URL for mutagen
// URL pattern: https://github.com/mutagen-io/mutagen/releases/download/v{version}/mutagen_{os}_{arch}_v{version}.tar.gz
func mutagenURLBuilder(template, version, goos, goarch string) string {
	// Mutagen uses standard os/arch naming (darwin, linux, amd64, arm64)
	return fmt.Sprintf(template, version, goos, goarch, version)
}
