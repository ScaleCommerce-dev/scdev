package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/create"
	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
	"github.com/spf13/cobra"
)

var renameForce bool

var renameCmd = &cobra.Command{
	Use:   "rename <new-name>",
	Short: "Rename the project",
	Long: `Rename the current project, migrating all Docker resources.

This stops the project, renames containers, volumes, and network,
updates the state file and link memberships, writes the new name
to .scdev/config.yaml, and restarts with the new name.

Volume data is preserved by copying to new volumes.
If the domain is auto-generated, the project URL will change.`,
	Args: cobra.ExactArgs(1),
	RunE: runRename,
}

func init() {
	renameCmd.Flags().BoolVarP(&renameForce, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	newName := args[0]

	// Validate new name (DNS-safe)
	if err := create.ValidateName(newName); err != nil {
		return err
	}

	// Load current project
	proj, err := project.Load()
	if err != nil {
		return err
	}

	oldName := proj.Config.Name

	if oldName == newName {
		return fmt.Errorf("project is already named %q", newName)
	}

	// Check new name not taken
	stateMgr, err := state.DefaultManager()
	if err != nil {
		return err
	}

	existing, err := stateMgr.GetProject(newName)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("a project named %q already exists at %s", newName, existing.Path)
	}

	// Show what will change
	globalCfg, _ := config.LoadGlobalConfig()
	domain := config.DefaultDomain
	if globalCfg != nil && globalCfg.Domain != "" {
		domain = globalCfg.Domain
	}
	protocol := "http"
	if globalCfg != nil && globalCfg.SSL.Enabled {
		protocol = "https"
	}

	fmt.Printf("Rename project: %s -> %s\n", oldName, newName)

	// Show domain change if auto-generated
	oldDomain := oldName + "." + domain
	newDomain := newName + "." + domain
	if proj.Config.Domain == oldDomain || proj.Config.Domain == "" {
		fmt.Printf("New URL: %s://%s\n", protocol, newDomain)
	}

	fmt.Println()
	fmt.Println("This will stop the project, migrate all volumes, and restart with the new name.")

	if !renameForce {
		if !confirm("Continue? [y/N]: ") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println()

	if err := proj.Rename(ctx, newName); err != nil {
		return err
	}

	// Update docs page
	updateDocsWithProjects(ctx)

	fmt.Println()
	fmt.Printf("Project renamed to %s\n", newName)

	// Show new project info
	reloaded, err := project.Load()
	if err == nil {
		fmt.Println()
		fmt.Println("Project Info:")
		fmt.Println()
		_ = showProjectInfo(ctx, reloaded)
	}

	return nil
}
