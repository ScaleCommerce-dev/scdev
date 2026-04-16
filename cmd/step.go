package cmd

import (
	"strings"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/ui"
	"github.com/spf13/cobra"
)

var stepCmd = &cobra.Command{
	Use:   "step <message>",
	Short: "Print a visually distinct status step",
	Long: `Print a visually distinct status step (two blank lines, colored prefix, bold text).

Intended for use inside .scdev/commands/*.just recipes so template progress
markers stand out against verbose nested command output. Example:

    setup:
        @scdev step "Installing PHP extensions"
        scdev exec app sh -c "apk add ..."

        @scdev step "Scaffolding project"
        scdev exec app composer create-project ...`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message := strings.Join(args, " ")

		plainMode := false
		if cfg, err := config.LoadGlobalConfig(); err == nil && cfg != nil {
			plainMode = ui.PlainMode(cfg.Terminal.Plain)
		}

		ui.StatusStep(message, plainMode)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stepCmd)
}
