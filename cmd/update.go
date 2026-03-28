package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update project to match config",
	Long: `Update the project to match the current configuration.

This command detects changes in the config and only recreates
containers that need to be updated. Use this after modifying
your config file instead of running 'scdev down && scdev start'.

Changes detected:
- Routing configuration (host_port, protocol, port)
- Image changes
- Environment variable changes
- Volume mount changes
- Command changes`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	proj, err := project.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Updating project %s...\n", proj.Config.Name)

	updated, err := proj.Update(ctx)
	if err != nil {
		return err
	}

	if updated {
		fmt.Printf("Project %s updated\n", proj.Config.Name)
	} else {
		fmt.Printf("Project %s is up to date\n", proj.Config.Name)
	}
	return nil
}
