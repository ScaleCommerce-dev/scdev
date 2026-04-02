package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
	"github.com/spf13/cobra"
)

var (
	cleanupForce  bool
	cleanupGlobal bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove project volumes",
	Long: `Remove Docker volumes for the current project.

By default, asks for confirmation before deleting each volume.
Use --force to skip confirmation prompts.
Use --global to clean up volumes from all registered projects and orphaned volumes.`,
	RunE: runCleanup,
}

func init() {
	cleanupCmd.Flags().BoolVarP(&cleanupForce, "force", "f", false, "skip confirmation prompts")
	cleanupCmd.Flags().BoolVar(&cleanupGlobal, "global", false, "clean up all registered projects and orphaned volumes")
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	if cleanupGlobal {
		return runGlobalCleanup(ctx)
	}

	return runProjectCleanup(ctx)
}

func runProjectCleanup(ctx context.Context) error {
	proj, err := project.Load()
	if err != nil {
		return err
	}

	volumes, err := proj.Volumes(ctx)
	if err != nil {
		return err
	}

	// Filter to only existing volumes
	var existingVolumes []string
	for _, vol := range volumes {
		if vol.Exists {
			existingVolumes = append(existingVolumes, vol.FullName)
		}
	}

	if len(existingVolumes) == 0 {
		fmt.Println("No volumes to clean up.")
		return nil
	}

	fmt.Printf("Found %d volume(s) for project %s:\n\n", len(existingVolumes), proj.Config.Name)
	for _, name := range existingVolumes {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println()

	if !confirmCleanup(len(existingVolumes)) {
		return nil
	}

	return deleteVolumes(ctx, proj.Runtime, existingVolumes)
}

func runGlobalCleanup(ctx context.Context) error {
	stateMgr, err := state.DefaultManager()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	projects, err := stateMgr.ListProjects()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	docker := runtime.NewDockerCLI()

	// Collect all volumes from registered projects
	knownVolumes := make(map[string]bool)
	existingVolumes := make(map[string]bool)

	for _, entry := range projects {
		proj, err := project.LoadFromDir(entry.Path)
		if err != nil {
			continue
		}

		volumes, err := proj.Volumes(ctx)
		if err != nil {
			continue
		}

		for _, vol := range volumes {
			knownVolumes[vol.FullName] = true
			if vol.Exists {
				existingVolumes[vol.FullName] = true
			}
		}
	}

	// Convert to sorted slice
	var projectVolumes []string
	for name := range existingVolumes {
		projectVolumes = append(projectVolumes, name)
	}

	// Find orphaned volumes (exist in Docker but not in any registered project)
	dockerVolumes, err := docker.ListVolumes(ctx, "name=.scdev")
	if err != nil {
		return fmt.Errorf("failed to list Docker volumes: %w", err)
	}

	var orphanVolumes []string
	for _, vol := range dockerVolumes {
		if !knownVolumes[vol.Name] {
			orphanVolumes = append(orphanVolumes, vol.Name)
		}
	}

	// Sort for consistent output
	sort.Strings(projectVolumes)
	sort.Strings(orphanVolumes)

	if len(projectVolumes) == 0 && len(orphanVolumes) == 0 {
		fmt.Println("No volumes to clean up.")
		return nil
	}

	// Display volumes
	if len(projectVolumes) > 0 {
		fmt.Printf("Project volumes (%d):\n", len(projectVolumes))
		for _, name := range projectVolumes {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()
	}

	if len(orphanVolumes) > 0 {
		fmt.Printf("Orphaned volumes (%d):\n", len(orphanVolumes))
		for _, name := range orphanVolumes {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()
	}

	allVolumes := make([]string, 0, len(projectVolumes)+len(orphanVolumes))
	allVolumes = append(allVolumes, projectVolumes...)
	allVolumes = append(allVolumes, orphanVolumes...)
	if !confirmCleanup(len(allVolumes)) {
		return nil
	}

	return deleteVolumes(ctx, docker, allVolumes)
}

func confirmCleanup(count int) bool {
	if cleanupForce {
		return true
	}

	if !confirm(fmt.Sprintf("Delete %d volume(s)? [y/N]: ", count)) {
		fmt.Println("Aborted.")
		return false
	}
	return true
}

type volumeRemover interface {
	RemoveVolume(ctx context.Context, name string) error
}

func deleteVolumes(ctx context.Context, remover volumeRemover, volumes []string) error {
	var deleted, failed int
	for _, name := range volumes {
		fmt.Printf("Removing %s... ", name)
		if err := remover.RemoveVolume(ctx, name); err != nil {
			fmt.Printf("failed: %v\n", err)
			failed++
		} else {
			fmt.Println("done")
			deleted++
		}
	}

	fmt.Printf("\nRemoved %d volume(s)", deleted)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println()

	return nil
}
