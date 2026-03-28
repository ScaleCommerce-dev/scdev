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

func runServicesStart(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)

	// Start router
	if err := mgr.StartRouter(ctx); err != nil {
		return err
	}

	// Start mail
	if err := mgr.StartMail(ctx); err != nil {
		return err
	}

	// Start db
	if err := mgr.StartDBUI(ctx); err != nil {
		return err
	}

	// Start redis insights
	if err := mgr.StartRedisInsights(ctx); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Shared services started:")
	protocol := "http"
	if cfg.SSL.Enabled {
		protocol = "https"
	}
	fmt.Printf("  Docs:   %s://docs.shared.%s\n", protocol, cfg.Domain)
	fmt.Printf("  Router: %s://router.shared.%s\n", protocol, cfg.Domain)
	fmt.Printf("  Mail:   %s://mail.shared.%s\n", protocol, cfg.Domain)
	fmt.Printf("  DB:     %s://db.shared.%s\n", protocol, cfg.Domain)
	fmt.Printf("  Redis:  %s://redis.shared.%s\n", protocol, cfg.Domain)

	return nil
}

func runServicesStop(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)

	// Stop redis insights first
	if err := mgr.StopRedisInsights(ctx); err != nil {
		return err
	}

	// Stop db
	if err := mgr.StopDBUI(ctx); err != nil {
		return err
	}

	// Stop mail
	if err := mgr.StopMail(ctx); err != nil {
		return err
	}

	// Stop router last
	if err := mgr.StopRouter(ctx); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Shared services stopped")
	return nil
}

func runServicesStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)

	routerStatus, err := mgr.RouterStatus(ctx)
	if err != nil {
		return err
	}

	mailStatus, err := mgr.MailStatus(ctx)
	if err != nil {
		return err
	}

	dbStatus, err := mgr.DBUIStatus(ctx)
	if err != nil {
		return err
	}

	redisStatus, err := mgr.RedisInsightsStatus(ctx)
	if err != nil {
		return err
	}

	fmt.Println("Shared Services Status")
	fmt.Println("======================")
	fmt.Println()

	protocol := "http"
	if cfg.SSL.Enabled {
		protocol = "https"
	}

	if routerStatus.Running {
		fmt.Printf("Docs:   running (%s://docs.shared.%s)\n", protocol, cfg.Domain)
		fmt.Printf("Router: running (%s://router.shared.%s)\n", protocol, cfg.Domain)
	} else {
		fmt.Println("Docs:   stopped (requires router)")
		fmt.Println("Router: stopped")
	}

	if mailStatus.Running {
		fmt.Printf("Mail:   running (%s://mail.shared.%s)\n", protocol, cfg.Domain)
	} else {
		fmt.Println("Mail:   stopped")
	}

	if dbStatus.Running {
		fmt.Printf("DB:     running (%s://db.shared.%s)\n", protocol, cfg.Domain)
	} else {
		fmt.Println("DB:     stopped")
	}

	if redisStatus.Running {
		fmt.Printf("Redis:  running (%s://redis.shared.%s)\n", protocol, cfg.Domain)
	} else {
		fmt.Println("Redis:  stopped")
	}

	return nil
}

func runServicesRecreate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)
	docker := runtime.NewDockerCLI()

	fmt.Println("Recreating shared services...")
	fmt.Println()

	// Stop all services first
	fmt.Println("Stopping services...")
	_ = mgr.StopRedisInsights(ctx)
	_ = mgr.StopDBUI(ctx)
	_ = mgr.StopMail(ctx)
	_ = mgr.StopRouter(ctx)

	// Remove containers
	fmt.Println("Removing containers...")
	_ = docker.RemoveContainer(ctx, services.RedisInsightsContainerName)
	_ = docker.RemoveContainer(ctx, services.DBUIContainerName)
	_ = docker.RemoveContainer(ctx, services.MailContainerName)
	_ = docker.RemoveContainer(ctx, services.RouterContainerName)

	// Start fresh
	fmt.Println("Starting services...")
	if err := mgr.StartRouter(ctx); err != nil {
		return err
	}
	if err := mgr.StartMail(ctx); err != nil {
		return err
	}
	if err := mgr.StartDBUI(ctx); err != nil {
		return err
	}
	if err := mgr.StartRedisInsights(ctx); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Shared services recreated:")
	protocol := "http"
	if cfg.SSL.Enabled {
		protocol = "https"
	}
	fmt.Printf("  Docs:   %s://docs.shared.%s\n", protocol, cfg.Domain)
	fmt.Printf("  Router: %s://router.shared.%s\n", protocol, cfg.Domain)
	fmt.Printf("  Mail:   %s://mail.shared.%s\n", protocol, cfg.Domain)
	fmt.Printf("  DB:     %s://db.shared.%s\n", protocol, cfg.Domain)
	fmt.Printf("  Redis:  %s://redis.shared.%s\n", protocol, cfg.Domain)

	return nil
}
