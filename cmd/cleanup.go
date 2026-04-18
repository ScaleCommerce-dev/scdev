package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
	"github.com/spf13/cobra"
)

var cleanupForce bool

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove unused containers, volumes and stale project registrations",
	Long: `Prune resources no longer associated with any live project:

  - Orphaned Docker containers (labelled scdev.project but no matching registration)
  - Orphaned Docker volumes (not owned by any registered project)
  - Stale state entries whose project directory no longer exists on disk

Resources belonging to still-registered projects are never touched - remove those
explicitly with scdev remove.

Use --force to skip the confirmation prompt.`,
	RunE: runCleanup,
}

func init() {
	cleanupCmd.Flags().BoolVarP(&cleanupForce, "force", "f", false, "skip confirmation prompt")
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	return withDocker(2*time.Minute, func(ctx context.Context) error {
		stateMgr, err := state.DefaultManager()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		projects, err := stateMgr.ListProjects()
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		type staleProject struct {
			name string
			path string
		}
		var staleProjects []staleProject
		liveNames := make(map[string]bool)

		for name, entry := range projects {
			if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
				staleProjects = append(staleProjects, staleProject{name: name, path: entry.Path})
				continue
			}
			liveNames[name] = true
		}

		docker := runtime.NewDockerCLI()

		containers, err := docker.ListContainers(ctx, "label=scdev.project")
		if err != nil {
			return fmt.Errorf("failed to list Docker containers: %w", err)
		}

		var orphanContainers []string
		for _, name := range containers {
			if !ownedByLiveProject(name, liveNames) {
				orphanContainers = append(orphanContainers, name)
			}
		}

		dockerVolumes, err := docker.ListVolumes(ctx, "name=.scdev")
		if err != nil {
			return fmt.Errorf("failed to list Docker volumes: %w", err)
		}

		var orphanVolumes []string
		for _, vol := range dockerVolumes {
			if !ownedByLiveProject(vol.Name, liveNames) {
				orphanVolumes = append(orphanVolumes, vol.Name)
			}
		}

		sort.Slice(staleProjects, func(i, j int) bool { return staleProjects[i].name < staleProjects[j].name })
		sort.Strings(orphanContainers)
		sort.Strings(orphanVolumes)

		if len(staleProjects) == 0 && len(orphanContainers) == 0 && len(orphanVolumes) == 0 {
			fmt.Println("Nothing to clean up.")
			return nil
		}

		if len(staleProjects) > 0 {
			fmt.Printf("Stale project registrations (%d) - directory missing on disk:\n", len(staleProjects))
			for _, p := range staleProjects {
				fmt.Printf("  - %s (%s)\n", p.name, p.path)
			}
			fmt.Println()
		}

		if len(orphanContainers) > 0 {
			fmt.Printf("Orphaned containers (%d) - not owned by any registered project:\n", len(orphanContainers))
			for _, name := range orphanContainers {
				fmt.Printf("  - %s\n", name)
			}
			fmt.Println()
		}

		if len(orphanVolumes) > 0 {
			fmt.Printf("Orphaned volumes (%d) - not owned by any registered project:\n", len(orphanVolumes))
			for _, name := range orphanVolumes {
				fmt.Printf("  - %s\n", name)
			}
			fmt.Println()
		}

		if !cleanupForce {
			if !confirm("Delete the items above? [y/N]: ") {
				fmt.Println("Aborted.")
				return nil
			}
		}

		for _, p := range staleProjects {
			fmt.Printf("Unregistering %s... ", p.name)
			if err := stateMgr.UnregisterProject(p.name); err != nil {
				fmt.Printf("failed: %v\n", err)
			} else {
				fmt.Println("done")
			}
		}

		for _, name := range orphanContainers {
			fmt.Printf("Removing container %s... ", name)
			if err := docker.ForceRemoveContainer(ctx, name); err != nil {
				fmt.Printf("failed: %v\n", err)
			} else {
				fmt.Println("done")
			}
		}

		var deleted, failed int
		for _, name := range orphanVolumes {
			fmt.Printf("Removing volume %s... ", name)
			if err := docker.RemoveVolume(ctx, name); err != nil {
				fmt.Printf("failed: %v\n", err)
				failed++
			} else {
				fmt.Println("done")
				deleted++
			}
		}

		if len(orphanVolumes) > 0 {
			fmt.Printf("\nRemoved %d volume(s)", deleted)
			if failed > 0 {
				fmt.Printf(", %d failed", failed)
			}
			fmt.Println()
		}

		return nil
	})
}

// ownedByLiveProject reports whether a scdev resource name
// (<base>.<projectname>.scdev - shared by containers and volumes)
// belongs to a currently-registered project whose directory still
// exists on disk.
func ownedByLiveProject(name string, liveNames map[string]bool) bool {
	if !strings.HasSuffix(name, ".scdev") {
		return false
	}
	trimmed := strings.TrimSuffix(name, ".scdev")
	dot := strings.LastIndex(trimmed, ".")
	if dot < 0 {
		return false
	}
	return liveNames[trimmed[dot+1:]]
}
