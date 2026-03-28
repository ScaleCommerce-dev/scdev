package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered projects",
	Long:  `Show all projects that have been started with scdev, along with their current status.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stateMgr, err := state.DefaultManager()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	projects, err := stateMgr.ListProjects()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects registered. Start a project with 'scdev start' to register it.")
		return nil
	}

	// Sort projects by name for consistent output
	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("%-20s %-12s %s\n", "NAME", "STATUS", "PATH")
	fmt.Printf("%-20s %-12s %s\n", "----", "------", "----")

	for _, name := range names {
		entry := projects[name]
		status := getProjectStatus(ctx, name, entry.Path)
		fmt.Printf("%-20s %-12s %s\n", name, status, entry.Path)
	}

	return nil
}

// getProjectStatus checks if any containers for the project are running
func getProjectStatus(ctx context.Context, name, path string) string {
	// Try to load project from its path
	proj, err := project.LoadFromDir(path)
	if err != nil {
		return "not found"
	}

	// Check if any service is running
	hasRunning := false
	hasStopped := false
	hasCreated := false

	for serviceName := range proj.Config.Services {
		containerName := proj.ContainerName(serviceName)

		exists, err := proj.Runtime.ContainerExists(ctx, containerName)
		if err != nil {
			continue
		}

		if !exists {
			continue
		}

		hasCreated = true

		running, err := proj.Runtime.IsContainerRunning(ctx, containerName)
		if err != nil {
			continue
		}

		if running {
			hasRunning = true
		} else {
			hasStopped = true
		}
	}

	if hasRunning {
		return "running"
	}
	if hasStopped {
		return "stopped"
	}
	if hasCreated {
		return "stopped"
	}
	return "not created"
}
