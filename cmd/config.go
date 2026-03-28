package cmd

import (
	"fmt"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show resolved configuration",
	Long:  `Show the project configuration with all variables expanded.`,
	RunE:  runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	proj, err := project.Load()
	if err != nil {
		return err
	}

	// Marshal back to YAML to show resolved config
	data, err := yaml.Marshal(proj.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
