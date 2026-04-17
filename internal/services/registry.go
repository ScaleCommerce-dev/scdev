package services

import (
	"context"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
)

// SharedServiceDef is the single source of truth for a shared service:
// everything needed to start, stop, inspect, and connect/disconnect it
// from project networks. Both the CLI (scdev services start) and the
// per-project connect flow iterate over the same slice so adding a new
// shared service is a single-file change here.
type SharedServiceDef struct {
	// Name is the human-readable display name (e.g., "Router").
	Name string

	// Subdomain is the URL prefix under {Subdomain}.{Domain} (e.g., "router.shared").
	Subdomain string

	// ContainerName is the Docker container name (e.g., "scdev_router").
	ContainerName string

	// Start, Stop, and Status manage the shared container lifecycle.
	Start  func(context.Context, *Manager) error
	Stop   func(context.Context, *Manager) error
	Status func(context.Context, *Manager) (*ServiceStatus, error)

	// Connect/Disconnect attach or detach the shared container to a
	// project's network. Called by internal/project when a project starts
	// or stops, for every service where ProjectEnabled returns true.
	Connect    func(ctx context.Context, m *Manager, projectNetwork string) error
	Disconnect func(ctx context.Context, m *Manager, projectNetwork string) error

	// ProjectEnabled reports whether a project wants this shared service,
	// reading the per-project config flag.
	ProjectEnabled func(*config.ProjectSharedConfig) bool
}

// AllSharedServices returns the ordered list of shared services. Order
// matters for lifecycle: start router first (others route through it),
// stop/disconnect router last.
func AllSharedServices() []SharedServiceDef {
	return []SharedServiceDef{
		{
			Name:          "Router",
			Subdomain:     "router.shared",
			ContainerName: RouterContainerName,
			Start:         func(ctx context.Context, m *Manager) error { return m.StartRouter(ctx) },
			Stop:          func(ctx context.Context, m *Manager) error { return m.StopRouter(ctx) },
			Status:        func(ctx context.Context, m *Manager) (*ServiceStatus, error) { return m.RouterStatus(ctx) },
			Connect:       func(ctx context.Context, m *Manager, net string) error { return m.ConnectRouterToProject(ctx, net) },
			Disconnect:    func(ctx context.Context, m *Manager, net string) error { return m.DisconnectRouterFromProject(ctx, net) },
			ProjectEnabled: func(s *config.ProjectSharedConfig) bool { return s.Router },
		},
		{
			Name:          "Mail",
			Subdomain:     "mail.shared",
			ContainerName: MailContainerName,
			Start:         func(ctx context.Context, m *Manager) error { return m.StartMail(ctx) },
			Stop:          func(ctx context.Context, m *Manager) error { return m.StopMail(ctx) },
			Status:        func(ctx context.Context, m *Manager) (*ServiceStatus, error) { return m.MailStatus(ctx) },
			Connect:       func(ctx context.Context, m *Manager, net string) error { return m.ConnectMailToProject(ctx, net) },
			Disconnect:    func(ctx context.Context, m *Manager, net string) error { return m.DisconnectMailFromProject(ctx, net) },
			ProjectEnabled: func(s *config.ProjectSharedConfig) bool { return s.Mail },
		},
		{
			Name:          "DB",
			Subdomain:     "db.shared",
			ContainerName: DBUIContainerName,
			Start:         func(ctx context.Context, m *Manager) error { return m.StartDBUI(ctx) },
			Stop:          func(ctx context.Context, m *Manager) error { return m.StopDBUI(ctx) },
			Status:        func(ctx context.Context, m *Manager) (*ServiceStatus, error) { return m.DBUIStatus(ctx) },
			Connect:       func(ctx context.Context, m *Manager, net string) error { return m.ConnectDBUIToProject(ctx, net) },
			Disconnect:    func(ctx context.Context, m *Manager, net string) error { return m.DisconnectDBUIFromProject(ctx, net) },
			ProjectEnabled: func(s *config.ProjectSharedConfig) bool { return s.DBUI },
		},
		{
			Name:          "Redis",
			Subdomain:     "redis.shared",
			ContainerName: RedisInsightsContainerName,
			Start:         func(ctx context.Context, m *Manager) error { return m.StartRedisInsights(ctx) },
			Stop:          func(ctx context.Context, m *Manager) error { return m.StopRedisInsights(ctx) },
			Status:        func(ctx context.Context, m *Manager) (*ServiceStatus, error) { return m.RedisInsightsStatus(ctx) },
			Connect:       func(ctx context.Context, m *Manager, net string) error { return m.ConnectRedisInsightsToProject(ctx, net) },
			Disconnect:    func(ctx context.Context, m *Manager, net string) error { return m.DisconnectRedisInsightsFromProject(ctx, net) },
			ProjectEnabled: func(s *config.ProjectSharedConfig) bool { return s.RedisInsights },
		},
	}
}
