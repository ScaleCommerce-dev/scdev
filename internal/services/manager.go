package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
)

const (
	// SharedNetworkName is the name of the shared Docker network
	SharedNetworkName = "scdev_shared"

	// RouterContainerName is the name of the Traefik router container
	RouterContainerName = "scdev_router"
)

// ServiceStatus represents the status of a shared service
type ServiceStatus struct {
	Running           bool
	ContainerID       string
	ConnectedNetworks []string
}

// Manager handles shared services (router, mail, etc.)
type Manager struct {
	cfg     *config.GlobalConfig
	runtime runtime.Runtime
}

// NewManager creates a new shared services manager
func NewManager(cfg *config.GlobalConfig) *Manager {
	return &Manager{
		cfg:     cfg,
		runtime: runtime.NewDockerCLI(),
	}
}

// EnsureSharedNetwork creates the shared network if it doesn't exist
func (m *Manager) EnsureSharedNetwork(ctx context.Context) error {
	exists, err := m.runtime.NetworkExists(ctx, SharedNetworkName)
	if err != nil {
		return fmt.Errorf("failed to check shared network: %w", err)
	}

	if !exists {
		fmt.Printf("Creating shared network %s...\n", SharedNetworkName)
		if err := m.runtime.CreateNetwork(ctx, SharedNetworkName); err != nil {
			return fmt.Errorf("failed to create shared network: %w", err)
		}
	}

	return nil
}

// =============================================================================
// Generic Service Helpers
// =============================================================================

// getServiceStatus returns the status of a service by container name
func (m *Manager) getServiceStatus(ctx context.Context, containerName, serviceName string) (*ServiceStatus, error) {
	container, err := m.runtime.GetContainer(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to check %s container: %w", serviceName, err)
	}

	if container == nil {
		return &ServiceStatus{Running: false}, nil
	}

	return &ServiceStatus{
		Running:     container.Running,
		ContainerID: container.ID,
	}, nil
}

// stopService stops a service container
func (m *Manager) stopService(ctx context.Context, containerName, displayName string) error {
	container, err := m.runtime.GetContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to check %s container: %w", displayName, err)
	}

	if container == nil {
		fmt.Printf("%s is not running\n", displayName)
		return nil
	}

	if !container.Running {
		fmt.Printf("%s is already stopped\n", displayName)
		return nil
	}

	fmt.Printf("Stopping %s...\n", strings.ToLower(displayName))
	return m.runtime.StopContainer(ctx, containerName)
}

// startService starts a service container with the given config
func (m *Manager) startService(ctx context.Context, containerName, displayName, image string, configFn func() runtime.ContainerConfig) error {
	// Ensure shared network exists
	if err := m.EnsureSharedNetwork(ctx); err != nil {
		return err
	}

	// Check if container already exists
	container, err := m.runtime.GetContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to check %s container: %w", displayName, err)
	}

	if container != nil {
		if container.Running {
			fmt.Printf("%s is already running\n", displayName)
			return nil
		}
		// Container exists but not running, start it
		fmt.Printf("Starting %s...\n", strings.ToLower(displayName))
		return m.runtime.StartContainer(ctx, containerName)
	}

	// Pull image if needed
	imageExists, err := m.runtime.ImageExists(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to check %s image: %w", displayName, err)
	}

	if !imageExists {
		fmt.Printf("Pulling image %s...\n", image)
		if err := m.runtime.PullImage(ctx, image); err != nil {
			return fmt.Errorf("failed to pull %s image: %w", displayName, err)
		}
	}

	// Create container
	cfg := configFn()

	fmt.Printf("Creating %s container...\n", strings.ToLower(displayName))
	if _, err := m.runtime.CreateContainer(ctx, cfg); err != nil {
		return fmt.Errorf("failed to create %s container: %w", displayName, err)
	}

	fmt.Printf("Starting %s...\n", strings.ToLower(displayName))
	if err := m.runtime.StartContainer(ctx, containerName); err != nil {
		return fmt.Errorf("failed to start %s: %w", displayName, err)
	}

	return nil
}

// connectServiceToProject connects a service to a project network
func (m *Manager) connectServiceToProject(ctx context.Context, containerName, displayName, projectNetwork string, statusFn func(context.Context) (*ServiceStatus, error)) error {
	status, err := statusFn(ctx)
	if err != nil {
		return err
	}

	if !status.Running {
		return fmt.Errorf("%s is not running (start with: scdev services start)", strings.ToLower(displayName))
	}

	if err := m.runtime.NetworkConnect(ctx, projectNetwork, containerName); err != nil {
		// Ignore "already connected" errors
		errStr := err.Error()
		if strings.Contains(strings.ToLower(errStr), "already exists") || strings.Contains(strings.ToLower(errStr), "already connected") {
			return nil
		}
		return fmt.Errorf("failed to connect %s to network %s: %w", strings.ToLower(displayName), projectNetwork, err)
	}

	return nil
}

// disconnectServiceFromProject disconnects a service from a project network
func (m *Manager) disconnectServiceFromProject(ctx context.Context, containerName, displayName, projectNetwork string) error {
	container, err := m.runtime.GetContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to check %s container: %w", displayName, err)
	}

	if container == nil {
		return nil // No container, nothing to disconnect
	}

	return m.runtime.NetworkDisconnect(ctx, projectNetwork, containerName)
}

// =============================================================================
// Router (special handling for ports)
// =============================================================================

// StartRouter starts the Traefik router container.
// TCP/UDP ports are aggregated from the state file to ensure the router
// has all ports needed by all registered projects.
func (m *Manager) StartRouter(ctx context.Context) error {
	// Aggregate all ports from all projects in state
	stateMgr, err := state.DefaultManager()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	tcpPorts, udpPorts, err := stateMgr.GetAllRoutingPorts()
	if err != nil {
		return fmt.Errorf("failed to get routing ports from state: %w", err)
	}

	// Ensure shared network exists
	if err := m.EnsureSharedNetwork(ctx); err != nil {
		return err
	}

	// Check if router already exists
	container, err := m.runtime.GetContainer(ctx, RouterContainerName)
	if err != nil {
		return fmt.Errorf("failed to check router container: %w", err)
	}

	if container != nil {
		// Check if we need to recreate due to port changes
		needsRecreate := m.routerNeedsRecreate(ctx, tcpPorts, udpPorts)

		if needsRecreate {
			fmt.Println("Router needs new ports, recreating...")
			if err := m.runtime.StopContainer(ctx, RouterContainerName); err != nil {
				// Ignore stop errors
			}
			if err := m.runtime.RemoveContainer(ctx, RouterContainerName); err != nil {
				return fmt.Errorf("failed to remove router container: %w", err)
			}
		} else if container.Running {
			fmt.Println("Router is already running")
			return nil
		} else {
			// Container exists but not running, start it
			fmt.Println("Starting router...")
			return m.runtime.StartContainer(ctx, RouterContainerName)
		}
	}

	// Pull image if needed
	image := m.cfg.Shared.Router.Image
	imageExists, err := m.runtime.ImageExists(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to check router image: %w", err)
	}

	if !imageExists {
		fmt.Printf("Pulling image %s...\n", image)
		if err := m.runtime.PullImage(ctx, image); err != nil {
			return fmt.Errorf("failed to pull router image: %w", err)
		}
	}

	// Create router container with all aggregated ports
	routerCfg := RouterConfig{
		Image:     m.cfg.Shared.Router.Image,
		Dashboard: m.cfg.Shared.Router.Dashboard,
		Domain:    m.cfg.Domain,
		TCPPorts:  tcpPorts,
		UDPPorts:  udpPorts,
	}

	// Configure TLS if SSL is enabled and certs exist
	if m.cfg.SSL.Enabled {
		traefikDir, err := config.EnsureTraefikConfig()
		if err != nil {
			fmt.Printf("Warning: failed to configure TLS: %v\n", err)
		} else if traefikDir != "" {
			routerCfg.TLSCertDir = config.GetCertsDir()
			routerCfg.TLSConfigDir = traefikDir
		}
	}

	// Configure docs page (always enabled)
	docsDir, err := config.EnsureDocsConfig(m.cfg.Domain, m.cfg.SSL.Enabled)
	if err != nil {
		fmt.Printf("Warning: failed to configure docs: %v\n", err)
	} else {
		routerCfg.DocsDir = docsDir
		// Ensure TLSConfigDir is set even without TLS (needed for docs routing config)
		if routerCfg.TLSConfigDir == "" {
			routerCfg.TLSConfigDir = config.GetTraefikConfigDir()
		}
	}

	cfg := RouterContainerConfig(routerCfg)

	fmt.Println("Creating router container...")
	if _, err := m.runtime.CreateContainer(ctx, cfg); err != nil {
		return fmt.Errorf("failed to create router container: %w", err)
	}

	fmt.Println("Starting router...")
	if err := m.runtime.StartContainer(ctx, RouterContainerName); err != nil {
		return fmt.Errorf("failed to start router: %w", err)
	}

	return nil
}

// RefreshRouter rebuilds the router if the ports from state don't match current router
func (m *Manager) RefreshRouter(ctx context.Context) error {
	container, err := m.runtime.GetContainer(ctx, RouterContainerName)
	if err != nil {
		return fmt.Errorf("failed to check router container: %w", err)
	}

	if container == nil || !container.Running {
		return nil // Router not running, nothing to refresh
	}

	stateMgr, err := state.DefaultManager()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	tcpPorts, udpPorts, err := stateMgr.GetAllRoutingPorts()
	if err != nil {
		return fmt.Errorf("failed to get routing ports from state: %w", err)
	}

	labels, err := m.runtime.GetContainerLabels(ctx, RouterContainerName)
	if err != nil {
		return fmt.Errorf("failed to get router labels: %w", err)
	}

	currentTCP := labels["scdev.tcp-ports"]
	currentUDP := labels["scdev.udp-ports"]
	requiredTCP := intsToString(tcpPorts)
	requiredUDP := intsToString(udpPorts)

	if currentTCP == requiredTCP && currentUDP == requiredUDP {
		return nil
	}

	fmt.Println("Updating router ports...")
	_ = m.runtime.StopContainer(ctx, RouterContainerName)
	if err := m.runtime.RemoveContainer(ctx, RouterContainerName); err != nil {
		return fmt.Errorf("failed to remove router container: %w", err)
	}

	return m.StartRouter(ctx)
}

func (m *Manager) routerNeedsRecreate(ctx context.Context, tcpPorts, udpPorts []int) bool {
	labels, err := m.runtime.GetContainerLabels(ctx, RouterContainerName)
	if err != nil {
		return true
	}

	currentTCP := labels["scdev.tcp-ports"]
	currentUDP := labels["scdev.udp-ports"]
	requiredTCP := intsToString(tcpPorts)
	requiredUDP := intsToString(udpPorts)

	return !portsContained(currentTCP, requiredTCP) || !portsContained(currentUDP, requiredUDP)
}

func portsContained(current, required string) bool {
	if required == "" {
		return true
	}
	if current == "" {
		return false
	}
	return current == required || containsAllPorts(current, required)
}

func containsAllPorts(current, required string) bool {
	currentPorts := make(map[string]bool)
	for _, p := range strings.Split(current, ",") {
		if p != "" {
			currentPorts[p] = true
		}
	}
	for _, p := range strings.Split(required, ",") {
		if p != "" && !currentPorts[p] {
			return false
		}
	}
	return true
}

func (m *Manager) StopRouter(ctx context.Context) error {
	return m.stopService(ctx, RouterContainerName, "Router")
}

func (m *Manager) RouterStatus(ctx context.Context) (*ServiceStatus, error) {
	return m.getServiceStatus(ctx, RouterContainerName, "Router")
}

func (m *Manager) ConnectRouterToProject(ctx context.Context, projectNetwork string) error {
	return m.connectServiceToProject(ctx, RouterContainerName, "Router", projectNetwork, m.RouterStatus)
}

func (m *Manager) DisconnectRouterFromProject(ctx context.Context, projectNetwork string) error {
	return m.disconnectServiceFromProject(ctx, RouterContainerName, "Router", projectNetwork)
}

// =============================================================================
// Mail
// =============================================================================

func (m *Manager) StartMail(ctx context.Context) error {
	return m.startService(ctx, MailContainerName, "Mail", m.cfg.Shared.Mail.Image, func() runtime.ContainerConfig {
		return MailContainerConfig(MailServiceConfig{
			Image:      m.cfg.Shared.Mail.Image,
			Domain:     m.cfg.Domain,
			TLSEnabled: m.cfg.SSL.Enabled,
		})
	})
}

func (m *Manager) StopMail(ctx context.Context) error {
	return m.stopService(ctx, MailContainerName, "Mail")
}

func (m *Manager) MailStatus(ctx context.Context) (*ServiceStatus, error) {
	return m.getServiceStatus(ctx, MailContainerName, "Mail")
}

func (m *Manager) ConnectMailToProject(ctx context.Context, projectNetwork string) error {
	return m.connectServiceToProject(ctx, MailContainerName, "Mail", projectNetwork, m.MailStatus)
}

func (m *Manager) DisconnectMailFromProject(ctx context.Context, projectNetwork string) error {
	return m.disconnectServiceFromProject(ctx, MailContainerName, "Mail", projectNetwork)
}

// =============================================================================
// DBUI (Adminer) - special handling for config
// =============================================================================

func (m *Manager) StartDBUI(ctx context.Context) error {
	// Ensure Adminer config (servers.php) exists before starting
	adminerCfgDir, err := EnsureAdminerConfig(ctx)
	if err != nil {
		fmt.Printf("Warning: could not configure Adminer servers: %v\n", err)
		adminerCfgDir = ""
	}

	return m.startService(ctx, DBUIContainerName, "DBUI", m.cfg.Shared.DBUI.Image, func() runtime.ContainerConfig {
		return DBUIContainerConfig(DBUIServiceConfig{
			Image:         m.cfg.Shared.DBUI.Image,
			Domain:        m.cfg.Domain,
			TLSEnabled:    m.cfg.SSL.Enabled,
			AdminerCfgDir: adminerCfgDir,
		})
	})
}

func (m *Manager) StopDBUI(ctx context.Context) error {
	return m.stopService(ctx, DBUIContainerName, "DBUI")
}

func (m *Manager) DBUIStatus(ctx context.Context) (*ServiceStatus, error) {
	return m.getServiceStatus(ctx, DBUIContainerName, "DBUI")
}

func (m *Manager) ConnectDBUIToProject(ctx context.Context, projectNetwork string) error {
	err := m.connectServiceToProject(ctx, DBUIContainerName, "DBUI", projectNetwork, m.DBUIStatus)
	// Update Adminer servers list on connect
	_ = UpdateAdminerServers(ctx)
	return err
}

func (m *Manager) DisconnectDBUIFromProject(ctx context.Context, projectNetwork string) error {
	err := m.disconnectServiceFromProject(ctx, DBUIContainerName, "DBUI", projectNetwork)
	// Update Adminer servers list on disconnect
	_ = UpdateAdminerServers(ctx)
	return err
}

// =============================================================================
// Redis Insights
// =============================================================================

func (m *Manager) StartRedisInsights(ctx context.Context) error {
	return m.startService(ctx, RedisInsightsContainerName, "RedisInsights", m.cfg.Shared.RedisInsights.Image, func() runtime.ContainerConfig {
		return RedisInsightsContainerConfig(RedisInsightsServiceConfig{
			Image:      m.cfg.Shared.RedisInsights.Image,
			Domain:     m.cfg.Domain,
			TLSEnabled: m.cfg.SSL.Enabled,
		})
	})
}

func (m *Manager) StopRedisInsights(ctx context.Context) error {
	return m.stopService(ctx, RedisInsightsContainerName, "RedisInsights")
}

func (m *Manager) RedisInsightsStatus(ctx context.Context) (*ServiceStatus, error) {
	return m.getServiceStatus(ctx, RedisInsightsContainerName, "RedisInsights")
}

func (m *Manager) ConnectRedisInsightsToProject(ctx context.Context, projectNetwork string) error {
	return m.connectServiceToProject(ctx, RedisInsightsContainerName, "RedisInsights", projectNetwork, m.RedisInsightsStatus)
}

func (m *Manager) DisconnectRedisInsightsFromProject(ctx context.Context, projectNetwork string) error {
	return m.disconnectServiceFromProject(ctx, RedisInsightsContainerName, "RedisInsights", projectNetwork)
}
