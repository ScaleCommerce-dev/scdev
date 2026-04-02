package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	runtimePkg "github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/services"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
	"github.com/ScaleCommerce-DEV/scdev/internal/ui"
	"github.com/spf13/cobra"
)

// requireDocker checks that Docker is running and returns an error if not.
// Call this at the top of any command that needs Docker.
func requireDocker(ctx context.Context) error {
	docker := runtimePkg.NewDockerCLI()
	return docker.CheckAvailable(ctx)
}

// openSharedServiceURL opens a shared service URL in the browser
// serviceName is used for error messages (e.g., "mail", "db", "router")
// urlPath is the subdomain (e.g., "mail.shared", "db.shared", "docs.shared")
// statusFn returns the service status
func openSharedServiceURL(
	serviceName string,
	urlPath string,
	statusFn func(context.Context, *services.Manager) (*services.ServiceStatus, error),
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)

	status, err := statusFn(ctx, mgr)
	if err != nil {
		return err
	}

	if !status.Running {
		return fmt.Errorf("%s service is not running\n\nStart it with: scdev services start", serviceName)
	}

	protocol := "http"
	if cfg.SSL.Enabled {
		protocol = "https"
	}
	url := fmt.Sprintf("%s://%s.%s", protocol, urlPath, cfg.Domain)

	plainMode := ui.PlainMode(cfg.Terminal.Plain)
	fmt.Printf("Opening %s\n", ui.Hyperlink(url, url, plainMode))

	return openBrowser(url)
}

// confirm prompts the user with a message and returns true if they answer y/yes.
func confirm(msg string) bool {
	fmt.Print(msg)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("failed to read response: %v\n", err)
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// completeProjectNames provides shell completion for registered project names.
func completeProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	stateMgr, err := state.DefaultManager()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	projects, err := stateMgr.ListProjects()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var names []string
	for name := range projects {
		names = append(names, name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// openBrowser opens the given URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
