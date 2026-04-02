package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/services"
	"github.com/spf13/cobra"
)

// sharedServiceDef defines a shared service for the registry.
type sharedServiceDef struct {
	name          string // display name (e.g., "Router")
	subdomain     string // URL subdomain (e.g., "router.shared")
	containerName string // Docker container name
	startFn       func(context.Context, *services.Manager) error
	stopFn        func(context.Context, *services.Manager) error
	statusFn      func(context.Context, *services.Manager) (*services.ServiceStatus, error)
}

// sharedServiceRegistry returns the ordered list of shared services.
func sharedServiceRegistry() []sharedServiceDef {
	return []sharedServiceDef{
		{
			name: "Router", subdomain: "router.shared", containerName: services.RouterContainerName,
			startFn:  func(ctx context.Context, m *services.Manager) error { return m.StartRouter(ctx) },
			stopFn:   func(ctx context.Context, m *services.Manager) error { return m.StopRouter(ctx) },
			statusFn: func(ctx context.Context, m *services.Manager) (*services.ServiceStatus, error) { return m.RouterStatus(ctx) },
		},
		{
			name: "Mail", subdomain: "mail.shared", containerName: services.MailContainerName,
			startFn:  func(ctx context.Context, m *services.Manager) error { return m.StartMail(ctx) },
			stopFn:   func(ctx context.Context, m *services.Manager) error { return m.StopMail(ctx) },
			statusFn: func(ctx context.Context, m *services.Manager) (*services.ServiceStatus, error) { return m.MailStatus(ctx) },
		},
		{
			name: "DB", subdomain: "db.shared", containerName: services.DBUIContainerName,
			startFn:  func(ctx context.Context, m *services.Manager) error { return m.StartDBUI(ctx) },
			stopFn:   func(ctx context.Context, m *services.Manager) error { return m.StopDBUI(ctx) },
			statusFn: func(ctx context.Context, m *services.Manager) (*services.ServiceStatus, error) { return m.DBUIStatus(ctx) },
		},
		{
			name: "Redis", subdomain: "redis.shared", containerName: services.RedisInsightsContainerName,
			startFn:  func(ctx context.Context, m *services.Manager) error { return m.StartRedisInsights(ctx) },
			stopFn:   func(ctx context.Context, m *services.Manager) error { return m.StopRedisInsights(ctx) },
			statusFn: func(ctx context.Context, m *services.Manager) (*services.ServiceStatus, error) { return m.RedisInsightsStatus(ctx) },
		},
	}
}

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage shared services",
	Long:  `Manage shared infrastructure services like Traefik router, Mailpit, and Adminer.`,
}

var servicesStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start shared services",
	Long:  `Start shared services (router, mail, db). This creates the shared network and starts Traefik, Mailpit, and Adminer.`,
	RunE:  runServicesStart,
}

var servicesStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop shared services",
	Long:  `Stop the shared services (router, mail, db).`,
	RunE:  runServicesStop,
}

var servicesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show shared services status",
	Long:  `Show the status of shared services (router, mail, etc.).`,
	RunE:  runServicesStatus,
}

var servicesRecreateCmd = &cobra.Command{
	Use:   "recreate",
	Short: "Recreate shared services",
	Long:  `Stop, remove, and recreate all shared service containers. Use this after updating scdev or when containers need to be rebuilt with new configuration.`,
	RunE:  runServicesRecreate,
}

func init() {
	servicesCmd.AddCommand(servicesStartCmd)
	servicesCmd.AddCommand(servicesStopCmd)
	servicesCmd.AddCommand(servicesStatusCmd)
	servicesCmd.AddCommand(servicesRecreateCmd)
	rootCmd.AddCommand(servicesCmd)
}

func printSharedServiceURLs(cfg *config.GlobalConfig, header string) {
	protocol := "http"
	if cfg.SSL.Enabled {
		protocol = "https"
	}
	fmt.Println()
	fmt.Println(header)
	fmt.Printf("  Docs:   %s://docs.shared.%s\n", protocol, cfg.Domain)
	for _, svc := range sharedServiceRegistry() {
		fmt.Printf("  %-7s %s://%s.%s\n", svc.name+":", protocol, svc.subdomain, cfg.Domain)
	}
}

func runServicesStart(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)

	for _, svc := range sharedServiceRegistry() {
		if err := svc.startFn(ctx, mgr); err != nil {
			return err
		}
	}

	printSharedServiceURLs(cfg, "Shared services started:")
	return nil
}

func runServicesStop(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)

	// Stop in reverse order (router last)
	registry := sharedServiceRegistry()
	for i := len(registry) - 1; i >= 0; i-- {
		if err := registry[i].stopFn(ctx, mgr); err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Println("Shared services stopped")
	return nil
}

func runServicesStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)

	protocol := "http"
	if cfg.SSL.Enabled {
		protocol = "https"
	}

	fmt.Println("Shared Services Status")
	fmt.Println("======================")
	fmt.Println()

	// Docs status depends on router
	registry := sharedServiceRegistry()
	routerStatus, err := registry[0].statusFn(ctx, mgr)
	if err != nil {
		return err
	}
	if routerStatus.Running {
		fmt.Printf("Docs:   running (%s://docs.shared.%s)\n", protocol, cfg.Domain)
	} else {
		fmt.Println("Docs:   stopped (requires router)")
	}

	for _, svc := range registry {
		status, err := svc.statusFn(ctx, mgr)
		if err != nil {
			return err
		}
		if status.Running {
			fmt.Printf("%-7s running (%s://%s.%s)\n", svc.name+":", protocol, svc.subdomain, cfg.Domain)
		} else {
			fmt.Printf("%-7s stopped\n", svc.name+":")
		}
	}

	return nil
}

func runServicesRecreate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := requireDocker(ctx); err != nil {
		return err
	}

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)
	docker := runtime.NewDockerCLI()
	registry := sharedServiceRegistry()

	fmt.Println("Recreating shared services...")
	fmt.Println()

	// Stop all services (reverse order)
	fmt.Println("Stopping services...")
	for i := len(registry) - 1; i >= 0; i-- {
		_ = registry[i].stopFn(ctx, mgr)
	}

	// Remove containers (reverse order)
	fmt.Println("Removing containers...")
	for i := len(registry) - 1; i >= 0; i-- {
		_ = docker.RemoveContainer(ctx, registry[i].containerName)
	}

	// Start fresh
	fmt.Println("Starting services...")
	for _, svc := range registry {
		if err := svc.startFn(ctx, mgr); err != nil {
			return err
		}
	}

	printSharedServiceURLs(cfg, "Shared services recreated:")
	return nil
}
