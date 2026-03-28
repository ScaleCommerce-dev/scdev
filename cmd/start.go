package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the project",
	Long:  `Start all services defined in the project configuration.`,
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	// Check if first-run setup is needed
	if _, err := RunSystemcheckIfNeeded(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	proj, err := project.Load()
	if err != nil {
		return err
	}

	// Verify domain DNS resolves to 127.0.0.1
	if _, err := config.VerifyDomainDNS(proj.Config.Domain); err != nil {
		return fmt.Errorf("DNS verification failed: %w", err)
	}

	fmt.Printf("Starting project %s...\n", proj.Config.Name)

	if err := proj.Start(ctx); err != nil {
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

	// Auto-open project URL in browser if configured
	if proj.Config.AutoOpenAtStart {
		globalCfg, err := config.LoadGlobalConfig()
		if err == nil {
			protocol := "http"
			if globalCfg.SSL.Enabled {
				protocol = "https"
			}
			url := fmt.Sprintf("%s://%s", protocol, proj.Config.Domain)
			fmt.Printf("\nOpening %s\n", url)
			_ = openBrowser(url)
		}
	}

	return nil
}
