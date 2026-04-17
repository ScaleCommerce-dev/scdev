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
	return withProject(7*time.Minute, func(ctx context.Context, proj *project.Project) error {
		fmt.Printf("Restarting project %s...\n", proj.Config.Name)

		if err := proj.Restart(ctx); err != nil {
			return err
		}

		updateDocsWithProjects(ctx)

		fmt.Println()
		fmt.Println("Project Info:")
		fmt.Println()
		return showProjectInfo(ctx, proj)
	})
}
