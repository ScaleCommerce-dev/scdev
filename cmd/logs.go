package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsTail   int
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "View container logs",
	Long: `View logs from a service container.

If no service is specified, logs from the first service are shown.
Use -f to follow (stream) logs in real-time.
Use --tail to limit the number of lines shown.`,
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output (stream in real-time)")
	logsCmd.Flags().IntVar(&logsTail, "tail", 100, "Number of lines to show from end of logs (0 = all)")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	// logs has a unique lifecycle: no timeout (may run forever with -f)
	// and cancels on SIGINT/SIGTERM. withProject's fixed timeout doesn't fit.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	proj, err := project.Load()
	if err != nil {
		return err
	}

	var service string
	if len(args) > 0 {
		service = args[0]
	} else {
		services := proj.ServiceNames()
		if len(services) == 0 {
			return cmd.Help()
		}
		service = services[0]
	}

	return proj.Logs(ctx, service, project.LogsOptions{
		Follow: logsFollow,
		Tail:   logsTail,
	})
}
