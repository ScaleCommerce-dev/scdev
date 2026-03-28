package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/ScaleCommerce-DEV/scdev/internal/tools"
)

// findJustfile attempts to locate a justfile for the given command name
// Returns nil if not in a project directory or justfile doesn't exist
func findJustfile(name string) *project.JustfileInfo {
	// Try to find project directory
	dir, err := config.FindProjectDir()
	if err != nil {
		return nil // Not in a project, let Cobra handle it
	}

	// Check if justfile exists
	justfile, err := project.GetJustfileFromDir(dir, name)
	if err != nil || justfile == nil {
		return nil
	}

	return justfile
}

// runJustfile executes a justfile command
func runJustfile(justfile *project.JustfileInfo, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Ensure just is available
	toolMgr, err := tools.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize tool manager: %w", err)
	}

	justPath, err := toolMgr.EnsureTool(ctx, tools.JustTool())
	if err != nil {
		return fmt.Errorf("failed to ensure just is installed: %w", err)
	}

	// Load project for environment variables
	proj, err := project.Load()
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	just := tools.NewJust(justPath)

	// Build environment
	env := proj.BuildJustEnv()

	// Run justfile
	return just.Run(ctx, justfile.Path, args, env)
}
