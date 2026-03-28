package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the project",
	Long:  `Stop all running containers but keep them for quick restart.`,
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	proj, err := project.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Stopping project %s...\n", proj.Config.Name)

	if err := proj.Stop(ctx); err != nil {
		return err
	}

	// Update docs page with current project info
	updateDocsWithProjects(ctx)

	fmt.Printf("Project %s stopped\n", proj.Config.Name)
	return nil
}
