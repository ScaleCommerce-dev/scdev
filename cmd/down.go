package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/ScaleCommerce-DEV/scdev/internal/services"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
	"github.com/spf13/cobra"
)

var (
	downRemoveVolumes bool
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop and remove containers",
	Long:  `Stop and remove all project containers. Networks are also removed.`,
	RunE:  runDown,
}

func init() {
	downCmd.Flags().BoolVarP(&downRemoveVolumes, "volumes", "v", false, "Also remove volumes (respects persist_on_delete)")
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	proj, err := project.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Removing project %s...\n", proj.Config.Name)

	if err := proj.Down(ctx, downRemoveVolumes); err != nil {
		return err
	}

	// Unregister project from global state
	stateMgr, err := state.DefaultManager()
	if err != nil {
		fmt.Printf("Warning: could not update project registry: %v\n", err)
	} else {
		if err := stateMgr.UnregisterProject(proj.Config.Name); err != nil {
			fmt.Printf("Warning: could not unregister project: %v\n", err)
		}
	}

	// Refresh router to remove unused ports
	if proj.Config.Shared.Router {
		globalCfg, err := config.LoadGlobalConfig()
		if err == nil {
			mgr := services.NewManager(globalCfg)
			if err := mgr.RefreshRouter(ctx); err != nil {
				fmt.Printf("Warning: could not refresh router: %v\n", err)
			}
		}
	}

	// Update docs page with current project info
	updateDocsWithProjects(ctx)

	fmt.Printf("Project %s removed\n", proj.Config.Name)
	return nil
}
