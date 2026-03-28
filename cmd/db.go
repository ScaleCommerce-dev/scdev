package cmd

import (
	"context"

	"github.com/ScaleCommerce-DEV/scdev/internal/services"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Open Adminer database UI",
	Long:  `Open the Adminer web interface in your default browser. Adminer provides a lightweight database management UI for MySQL, PostgreSQL, SQLite, and more.`,
	RunE:  runDB,
}

func init() {
	rootCmd.AddCommand(dbCmd)
}

func runDB(cmd *cobra.Command, args []string) error {
	return openSharedServiceURL("db", "db.shared",
		func(ctx context.Context, mgr *services.Manager) (*services.ServiceStatus, error) {
			return mgr.DBUIStatus(ctx)
		},
	)
}
