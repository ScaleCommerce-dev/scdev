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

var volumesGlobal bool

var volumesCmd = &cobra.Command{
	Use:   "volumes [project]",
	Short: "List project volumes",
	Long: `List all volumes defined in the project configuration and their status.

Without arguments, shows volumes for the current project.
With a project name, shows volumes for that specific project.
With --global, shows volumes for all registered projects.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVolumes,
}

func init() {
	volumesCmd.Flags().BoolVar(&volumesGlobal, "global", false, "list volumes for all registered projects")
	rootCmd.AddCommand(volumesCmd)
}

func runVolumes(cmd *cobra.Command, args []string) error {
	return withDocker(30*time.Second, func(ctx context.Context) error {
		if volumesGlobal {
			return listGlobalVolumes(ctx)
		}
		if len(args) == 1 {
			return listProjectVolumesByName(ctx, args[0])
		}
		return listCurrentProjectVolumes(ctx)
	})
}

func listCurrentProjectVolumes(ctx context.Context) error {
	proj, err := project.Load()
	if err != nil {
		return err
	}

	return printProjectVolumes(ctx, proj)
}

func listProjectVolumesByName(ctx context.Context, name string) error {
	stateMgr, err := state.DefaultManager()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	entry, err := stateMgr.GetProject(name)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}
	if entry == nil {
		return fmt.Errorf("project %q not found in registry", name)
	}

	proj, err := project.LoadFromDir(entry.Path)
	if err != nil {
		return fmt.Errorf("failed to load project from %s: %w", entry.Path, err)
	}

	return printProjectVolumes(ctx, proj)
}

func listGlobalVolumes(ctx context.Context) error {
	stateMgr, err := state.DefaultManager()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	projects, err := stateMgr.ListProjects()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	// Track all known volume names from registered projects
	knownVolumes := make(map[string]bool)

	// Sort project names for consistent output
	projectNames := make([]string, 0, len(projects))
	for name := range projects {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	first := true
	for _, name := range projectNames {
		entry := projects[name]
		proj, err := project.LoadFromDir(entry.Path)
		if err != nil {
			fmt.Printf("# %s (error: %v)\n\n", name, err)
			continue
		}

		volumes, err := proj.Volumes(ctx)
		if err != nil {
			fmt.Printf("# %s (error loading volumes: %v)\n\n", name, err)
			continue
		}

		// Track all volume full names
		for _, vol := range volumes {
			knownVolumes[vol.FullName] = true
		}

		if len(volumes) == 0 {
			continue // Skip projects with no volumes in output
		}

		if !first {
			fmt.Println()
		}
		first = false

		fmt.Printf("# %s (%s)\n", name, entry.Path)
		fmt.Printf("%-20s %-40s %s\n", "NAME", "FULL NAME", "EXISTS")
		fmt.Printf("%-20s %-40s %s\n", "----", "---------", "------")

		for _, vol := range volumes {
			existsStr := "no"
			if vol.Exists {
				existsStr = "yes"
			}
			fmt.Printf("%-20s %-40s %s\n", vol.Name, vol.FullName, existsStr)
		}
	}

	// Check for orphaned volumes (exist in Docker but not in any registered project)
	docker := runtime.NewDockerCLI()
	dockerVolumes, err := docker.ListVolumes(ctx, "name=.scdev")
	if err != nil {
		// Non-fatal: just skip orphan detection
		return nil
	}

	var orphans []string
	for _, vol := range dockerVolumes {
		if !knownVolumes[vol.Name] {
			orphans = append(orphans, vol.Name)
		}
	}

	if len(orphans) > 0 {
		if !first {
			fmt.Println()
		}
		fmt.Println("# Orphaned volumes (not associated with any registered project)")
		fmt.Printf("%-40s\n", "VOLUME NAME")
		fmt.Printf("%-40s\n", "-----------")
		sort.Strings(orphans)
		for _, name := range orphans {
			fmt.Printf("%s\n", name)
		}
		fmt.Println()
		fmt.Println("To remove orphaned volumes: docker volume rm <name>")
	}

	if first && len(orphans) == 0 {
		fmt.Println("No volumes found.")
	}

	return nil
}

func printProjectVolumes(ctx context.Context, proj *project.Project) error {
	volumes, err := proj.Volumes(ctx)
	if err != nil {
		return err
	}

	if len(volumes) == 0 {
		fmt.Println("No volumes defined in project configuration")
		return nil
	}

	fmt.Printf("Volumes for project %s:\n\n", proj.Config.Name)
	fmt.Printf("%-20s %-40s %s\n", "NAME", "FULL NAME", "EXISTS")
	fmt.Printf("%-20s %-40s %s\n", "----", "---------", "------")

	for _, vol := range volumes {
		existsStr := "no"
		if vol.Exists {
			existsStr = "yes"
		}
		fmt.Printf("%-20s %-40s %s\n", vol.Name, vol.FullName, existsStr)
	}

	return nil
}
