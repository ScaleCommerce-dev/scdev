package cmd

import (
	"context"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/project"
	"github.com/spf13/cobra"
)

var (
	execUser    string
	execWorkdir string
)

var execCmd = &cobra.Command{
	Use:   "exec [flags] <service> <command> [args...]",
	Short: "Execute a command in a service container",
	Long:  `Execute a command in a running service container.`,
	RunE:  runExec,
}

func init() {
	execCmd.Flags().StringVarP(&execUser, "user", "u", "", "Username or UID to run command as")
	execCmd.Flags().StringVarP(&execWorkdir, "workdir", "w", "", "Working directory inside the container")
	execCmd.Flags().SetInterspersed(false) // Stop parsing flags after first positional arg
	rootCmd.AddCommand(execCmd)
}

func runExec(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return cmd.Help()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	proj, err := project.Load()
	if err != nil {
		return err
	}

	service := args[0]
	command := args[1:]

	opts := project.ExecOptions{
		User:    execUser,
		Workdir: execWorkdir,
	}

	return proj.Exec(ctx, service, command, true, opts)
}
