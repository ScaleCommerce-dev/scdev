package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

	service := args[0]
	command := args[1:]

	// Strip leading "--" separator if present (common pattern: scdev exec app -- cmd)
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}

	if len(command) == 0 {
		return cmd.Help()
	}

	err := withProject(30*time.Minute, func(ctx context.Context, proj *project.Project) error {
		opts := project.ExecOptions{
			User:    execUser,
			Workdir: execWorkdir,
		}
		return proj.Exec(ctx, service, command, true, opts)
	})

	// Propagate the child process's exit code without letting cobra print
	// "Error: exit status N". This matches how `docker exec` and shells behave:
	// a non-zero exit from the inner command is not an scdev failure.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return err
}
