package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/ScaleCommerce-DEV/scdev/internal/ui"
	"github.com/spf13/cobra"
)

var startQuiet bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the project",
	Long:  `Start all services defined in the project configuration.`,
	RunE:  runStart,
}

func init() {
	startCmd.Flags().BoolVarP(&startQuiet, "quiet", "q", false, "Skip project info display after start")
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	// Check if first-run setup is needed (before we touch Docker)
	if _, err := RunSystemcheckIfNeeded(); err != nil {
		return err
	}

	return withProject(5*time.Minute, runStartImpl)
}

func runStartImpl(ctx context.Context, proj *project.Project) error {
	// Verify domain DNS resolves to 127.0.0.1
	if _, err := config.VerifyDomainDNS(proj.Config.Domain); err != nil {
		return fmt.Errorf("DNS verification failed: %w", err)
	}

	plainMode := false
	if gcfg, err := config.LoadGlobalConfig(); err == nil && gcfg != nil {
		plainMode = ui.PlainMode(gcfg.Terminal.Plain)
	}
	ui.StatusStep(fmt.Sprintf("Starting project %s", proj.Config.Name), plainMode)

	if err := proj.Start(ctx); err != nil {
		return err
	}

	updateDocsWithProjects(ctx)

	if startQuiet {
		return nil
	}

	fmt.Println()
	fmt.Println("Project Info:")
	fmt.Println()
	if err := showProjectInfo(ctx, proj); err != nil {
		return err
	}

	if proj.Config.AutoOpenAtStart {
		if globalCfg, err := config.LoadGlobalConfig(); err == nil {
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
