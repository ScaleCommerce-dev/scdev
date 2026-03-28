package cmd

import (
	"context"

	"github.com/ScaleCommerce-DEV/scdev/internal/services"
	"github.com/spf13/cobra"
)

var mailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Open Mailpit web UI",
	Long:  `Open the Mailpit web interface in your default browser. Mailpit catches all emails sent by projects using SMTP.`,
	RunE:  runMail,
}

func init() {
	rootCmd.AddCommand(mailCmd)
}

func runMail(cmd *cobra.Command, args []string) error {
	return openSharedServiceURL("mail", "mail.shared",
		func(ctx context.Context, mgr *services.Manager) (*services.ServiceStatus, error) {
			return mgr.MailStatus(ctx)
		},
	)
}
