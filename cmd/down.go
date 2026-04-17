package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/spf13/cobra"
)

var (
	downRemoveVolumes bool
	downForce         bool
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop and remove containers",
	Long:  `Stop and remove all project containers. Networks are also removed.`,
	RunE:  runDown,
}

func init() {
	downCmd.Flags().BoolVarP(&downRemoveVolumes, "volumes", "v", false, "Also remove volumes")
	downCmd.Flags().BoolVarP(&downForce, "force", "f", false, "Skip confirmation prompt when removing volumes")
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	return withProject(2*time.Minute, func(ctx context.Context, proj *project.Project) error {
		if downRemoveVolumes && !downForce {
			msg := fmt.Sprintf("This will remove all containers, networks, and volumes for project %q.\nData stored in volumes will be permanently deleted. Continue? [y/N]: ", proj.Config.Name)
			if !confirm(msg) {
				fmt.Println("Aborted.")
				return nil
			}
		}

		fmt.Printf("Removing project %s...\n", proj.Config.Name)

		if err := proj.Down(ctx, downRemoveVolumes); err != nil {
			return err
		}

		updateDocsWithProjects(ctx)

		fmt.Printf("Project %s removed\n", proj.Config.Name)
		return nil
	})
}
