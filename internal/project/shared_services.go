package project

import (
	"context"
	"fmt"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/services"
)

// sharedServiceEntry pairs a config flag with its connect/disconnect methods.
type sharedServiceEntry struct {
	enabled    bool
	connect    func(context.Context) error
	disconnect func(context.Context)
}

// enabledSharedServices returns the list of shared services enabled for this project.
func (p *Project) enabledSharedServices() []sharedServiceEntry {
	return []sharedServiceEntry{
		{p.Config.Shared.Router, p.connectRouter, p.disconnectRouter},
		{p.Config.Shared.Mail, p.connectMail, p.disconnectMail},
		{p.Config.Shared.DBUI, p.connectDBUI, p.disconnectDBUI},
		{p.Config.Shared.RedisInsights, p.connectRedisInsights, p.disconnectRedisInsights},
	}
}

// connectEnabledSharedServices connects all enabled shared services to the project network.
func (p *Project) connectEnabledSharedServices(ctx context.Context) {
	for _, svc := range p.enabledSharedServices() {
		if svc.enabled {
			if err := svc.connect(ctx); err != nil {
				fmt.Printf("Warning: %v\n", err)
			}
		}
	}
}

// disconnectEnabledSharedServices disconnects all enabled shared services from the project network.
func (p *Project) disconnectEnabledSharedServices(ctx context.Context) {
	// Disconnect in reverse order (router last)
	entries := p.enabledSharedServices()
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].enabled {
			entries[i].disconnect(ctx)
		}
	}
}

// connectSharedService connects a shared service to this project's network
func (p *Project) connectSharedService(
	ctx context.Context,
	displayName string,
	startFn func(context.Context, *services.Manager) error,
	connectFn func(context.Context, *services.Manager, string) error,
) error {
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)

	fmt.Printf("Ensuring shared %s is running...\n", displayName)
	if err := startFn(ctx, mgr); err != nil {
		return fmt.Errorf("failed to start %s: %w", displayName, err)
	}

	return connectFn(ctx, mgr, p.NetworkName())
}

// disconnectSharedService disconnects a shared service from this project's network
func (p *Project) disconnectSharedService(
	ctx context.Context,
	disconnectFn func(context.Context, *services.Manager, string) error,
) {
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return // Ignore errors
	}

	mgr := services.NewManager(cfg)
	_ = disconnectFn(ctx, mgr, p.NetworkName())
}

// connectRouter connects the shared router to this project's network
func (p *Project) connectRouter(ctx context.Context) error {
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	mgr := services.NewManager(cfg)

	fmt.Println("Ensuring shared router is running...")
	if err := mgr.StartRouter(ctx); err != nil {
		return fmt.Errorf("failed to start router: %w", err)
	}

	return mgr.ConnectRouterToProject(ctx, p.NetworkName())
}

// disconnectRouter disconnects the shared router from this project's network
func (p *Project) disconnectRouter(ctx context.Context) {
	p.disconnectSharedService(ctx, func(ctx context.Context, mgr *services.Manager, network string) error {
		return mgr.DisconnectRouterFromProject(ctx, network)
	})
}

// connectMail connects the shared mail service to this project's network
func (p *Project) connectMail(ctx context.Context) error {
	return p.connectSharedService(ctx, "mail",
		func(ctx context.Context, mgr *services.Manager) error { return mgr.StartMail(ctx) },
		func(ctx context.Context, mgr *services.Manager, network string) error { return mgr.ConnectMailToProject(ctx, network) },
	)
}

// disconnectMail disconnects the shared mail service from this project's network
func (p *Project) disconnectMail(ctx context.Context) {
	p.disconnectSharedService(ctx, func(ctx context.Context, mgr *services.Manager, network string) error {
		return mgr.DisconnectMailFromProject(ctx, network)
	})
}

// connectDBUI connects the shared database UI service to this project's network
func (p *Project) connectDBUI(ctx context.Context) error {
	return p.connectSharedService(ctx, "DBUI",
		func(ctx context.Context, mgr *services.Manager) error { return mgr.StartDBUI(ctx) },
		func(ctx context.Context, mgr *services.Manager, network string) error { return mgr.ConnectDBUIToProject(ctx, network) },
	)
}

// disconnectDBUI disconnects the shared database UI service from this project's network
func (p *Project) disconnectDBUI(ctx context.Context) {
	p.disconnectSharedService(ctx, func(ctx context.Context, mgr *services.Manager, network string) error {
		return mgr.DisconnectDBUIFromProject(ctx, network)
	})
}

// connectRedisInsights connects the shared Redis Insights service to this project's network
func (p *Project) connectRedisInsights(ctx context.Context) error {
	return p.connectSharedService(ctx, "Redis Insights",
		func(ctx context.Context, mgr *services.Manager) error { return mgr.StartRedisInsights(ctx) },
		func(ctx context.Context, mgr *services.Manager, network string) error { return mgr.ConnectRedisInsightsToProject(ctx, network) },
	)
}

// disconnectRedisInsights disconnects the shared Redis Insights service from this project's network
func (p *Project) disconnectRedisInsights(ctx context.Context) {
	p.disconnectSharedService(ctx, func(ctx context.Context, mgr *services.Manager, network string) error {
		return mgr.DisconnectRedisInsightsFromProject(ctx, network)
	})
}

// configureRouting adds Traefik labels for routing based on the routing config
func (p *Project) configureRouting(cfg *runtime.ContainerConfig, serviceName string, routing *config.RoutingConfig, tlsEnabled bool) {
	traefikName := fmt.Sprintf("%s-%s", p.Config.Name, serviceName)

	cfg.Labels["traefik.enable"] = "true"
	cfg.Labels["traefik.docker.network"] = p.NetworkName()

	switch routing.Protocol {
	case "http":
		port := routing.Port
		if port == 0 {
			port = 80
		}
		// Always configure HTTP router
		cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.rule", traefikName)] = fmt.Sprintf("Host(`%s`)", p.Config.Domain)
		cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", traefikName)] = "http"
		cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.service", traefikName)] = traefikName
		cfg.Labels[fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", traefikName)] = fmt.Sprintf("%d", port)
		cfg.Labels[fmt.Sprintf("traefik.http.services.%s.loadbalancer.passHostHeader", traefikName)] = "true"

		// Also configure HTTPS router when TLS is enabled
		if tlsEnabled {
			httpsName := traefikName + "-https"
			cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.rule", httpsName)] = fmt.Sprintf("Host(`%s`)", p.Config.Domain)
			cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", httpsName)] = "https"
			cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.tls", httpsName)] = "true"
			cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.service", httpsName)] = traefikName // Use same service
		}

	case "https":
		port := routing.Port
		if port == 0 {
			port = 443
		}
		cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.rule", traefikName)] = fmt.Sprintf("Host(`%s`)", p.Config.Domain)
		cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", traefikName)] = "https"
		cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.tls", traefikName)] = "true"
		cfg.Labels[fmt.Sprintf("traefik.http.routers.%s.service", traefikName)] = traefikName
		cfg.Labels[fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", traefikName)] = fmt.Sprintf("%d", port)
		cfg.Labels[fmt.Sprintf("traefik.http.services.%s.loadbalancer.passHostHeader", traefikName)] = "true"

	case "tcp":
		if routing.Port == 0 || routing.HostPort == 0 {
			return // TCP requires both port and host_port
		}
		entrypoint := fmt.Sprintf("tcp-%d", routing.HostPort)
		cfg.Labels[fmt.Sprintf("traefik.tcp.routers.%s.rule", traefikName)] = "HostSNI(`*`)"
		cfg.Labels[fmt.Sprintf("traefik.tcp.routers.%s.entrypoints", traefikName)] = entrypoint
		cfg.Labels[fmt.Sprintf("traefik.tcp.routers.%s.service", traefikName)] = traefikName
		cfg.Labels[fmt.Sprintf("traefik.tcp.services.%s.loadbalancer.server.port", traefikName)] = fmt.Sprintf("%d", routing.Port)

	case "udp":
		if routing.Port == 0 || routing.HostPort == 0 {
			return // UDP requires both port and host_port
		}
		entrypoint := fmt.Sprintf("udp-%d", routing.HostPort)
		cfg.Labels[fmt.Sprintf("traefik.udp.routers.%s.entrypoints", traefikName)] = entrypoint
		cfg.Labels[fmt.Sprintf("traefik.udp.routers.%s.service", traefikName)] = traefikName
		cfg.Labels[fmt.Sprintf("traefik.udp.services.%s.loadbalancer.server.port", traefikName)] = fmt.Sprintf("%d", routing.Port)
	}
}

// GetRequiredPorts returns all TCP/UDP host ports required by this project's routing config
func (p *Project) GetRequiredPorts() (tcpPorts, udpPorts []int) {
	for _, svc := range p.Config.Services {
		if svc.Routing == nil || svc.Routing.HostPort == 0 {
			continue
		}
		switch svc.Routing.Protocol {
		case "tcp":
			tcpPorts = append(tcpPorts, svc.Routing.HostPort)
		case "udp":
			udpPorts = append(udpPorts, svc.Routing.HostPort)
		}
	}
	return
}
