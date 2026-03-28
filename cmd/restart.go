package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the project",
	Long:  `Stop and start all services in the project.`,
	RunE:  runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()

	proj, err := project.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Restarting project %s...\n", proj.Config.Name)

	if err := proj.Restart(ctx); err != nil {
		return err
	}

	// Update docs page with current project info
	updateDocsWithProjects(ctx)

	fmt.Println()
	fmt.Println("Project Info:")
	fmt.Println()
	if err := showProjectInfo(ctx, proj); err != nil {
		return err
	}

	return nil
}
