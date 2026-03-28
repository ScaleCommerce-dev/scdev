package project

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/mutagen"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/services"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
	"github.com/ScaleCommerce-DEV/scdev/internal/tools"
)

// Project represents a loaded scdev project
type Project struct {
	Dir     string
	Config  *config.ProjectConfig
	Runtime runtime.Runtime
}

// ExecOptions contains options for executing a command in a container
type ExecOptions struct {
	User    string // Username or UID to run command as
	Workdir string // Working directory inside the container
}

// Load finds and loads the project from the current directory
func Load() (*Project, error) {
	dir, err := config.FindProjectDir()
	if err != nil {
		return nil, err
	}

	return LoadFromDir(dir)
}

// LoadFromDir loads a project from a specific directory
func LoadFromDir(dir string) (*Project, error) {
	cfg, err := config.LoadProject(dir)
	if err != nil {
		return nil, err
	}

	return &Project{
		Dir:     dir,
		Config:  cfg,
		Runtime: runtime.NewDockerCLI(),
	}, nil
}

// ContainerName returns the full container name for a service
// Format: <service>.<project>.scdev (e.g., app.myproject.scdev)
func (p *Project) ContainerName(service string) string {
	return fmt.Sprintf("%s.%s.scdev", service, p.Config.Name)
}

// NetworkName returns the project network name
// Format: <project>.scdev (e.g., myproject.scdev)
func (p *Project) NetworkName() string {
	return fmt.Sprintf("%s.scdev", p.Config.Name)
}

// VolumeName returns the full volume name for a project volume
// Format: <volume>.<project>.scdev (e.g., db_data.myproject.scdev)
func (p *Project) VolumeName(volume string) string {
	return fmt.Sprintf("%s.%s.scdev", volume, p.Config.Name)
}

// ContainerStatus returns the status of a container: "running", "stopped", "not created", or "unknown"
func (p *Project) ContainerStatus(ctx context.Context, containerName string) string {
	exists, err := p.Runtime.ContainerExists(ctx, containerName)
	if err != nil || !exists {
		return "not created"
	}

	running, err := p.Runtime.IsContainerRunning(ctx, containerName)
	if err != nil {
		return "unknown"
	}

	if running {
		return "running"
	}
	return "stopped"
}

// isTLSAvailable checks if TLS is enabled in config and certs exist
func isTLSAvailable() bool {
	globalCfg, err := config.LoadGlobalConfig()
	if err != nil || !globalCfg.SSL.Enabled {
		return false
	}

	// Check if certs exist
	certsDir := config.GetCertsDir()
	certPath := filepath.Join(certsDir, "cert.pem")
	keyPath := filepath.Join(certsDir, "key.pem")

	if _, err := os.Stat(certPath); err != nil {
		return false
	}
	if _, err := os.Stat(keyPath); err != nil {
		return false
	}

	return true
}

// parseVolumeMount parses a volume string like "db_data:/var/lib/data" or "/host/path:/container/path"
// Returns (source, target, isNamedVolume)
func parseVolumeMount(volume string) (source, target string, isNamedVolume bool) {
	parts := strings.SplitN(volume, ":", 2)
	if len(parts) != 2 {
		return volume, volume, false
	}

	source = parts[0]
	target = parts[1]

	// If source starts with / or . it's a bind mount, otherwise it's a named volume
	isNamedVolume = !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, ".")

	return source, target, isNamedVolume
}

// checkPortAvailability checks if all configured routing ports are available
func (p *Project) checkPortAvailability(ctx context.Context) error {
	if !p.Config.Shared.Router {
		return nil // No routing ports to check if not using shared router
	}

	stateMgr, err := state.DefaultManager()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	for serviceName, svc := range p.Config.Services {
		if svc.Routing == nil || svc.Routing.HostPort == 0 {
			continue
		}

		port := svc.Routing.HostPort
		protocol := svc.Routing.Protocol

		// Check state file for port ownership
		var owner string
		if protocol == "tcp" {
			owner, err = stateMgr.GetTCPPortOwner(port)
		} else if protocol == "udp" {
			owner, err = stateMgr.GetUDPPortOwner(port)
		}
		if err != nil {
			return fmt.Errorf("failed to check port ownership: %w", err)
		}

		// If owned by current project, it's OK (restart scenario)
		if owner == p.Config.Name {
			continue
		}

		// If owned by another project, give specific error
		if owner != "" {
			return fmt.Errorf("service %s: port %d is already used by project '%s'\nStop that project or choose a different host_port",
				serviceName, port, owner)
		}

		// Not owned by any project - check if port is available on host
		// (could be used by another Docker container or system service)
		hostPort := fmt.Sprintf("0.0.0.0:%d", port)
		if !isPortAvailable(hostPort) {
			return fmt.Errorf("service %s: port %d is already in use on your system\nStop the process using this port or choose a different host_port",
				serviceName, port)
		}
	}
	return nil
}


// isPortAvailable checks if a port is available for binding
func isPortAvailable(hostPort string) bool {
	ln, err := net.Listen("tcp", hostPort)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// Start starts all project services
func (p *Project) Start(ctx context.Context) error {
	// Check port availability before starting anything
	if err := p.checkPortAvailability(ctx); err != nil {
		return err
	}

	// Check if Mutagen is enabled
	mutagenEnabled := p.IsMutagenEnabled()
	var m *mutagen.Mutagen
	var mutagenMounts []MutagenSyncMount

	if mutagenEnabled {
		var err error
		m, err = p.EnsureMutagen(ctx)
		if err != nil {
			return fmt.Errorf("failed to initialize Mutagen: %w", err)
		}
		mutagenMounts = p.GetMutagenSyncMounts()

		// Create Mutagen sync volumes
		if err := p.createMutagenVolumes(ctx, mutagenMounts); err != nil {
			return err
		}
	}

	// Create project network if it doesn't exist
	networkName := p.NetworkName()
	networkExists, err := p.Runtime.NetworkExists(ctx, networkName)
	if err != nil {
		return fmt.Errorf("failed to check network: %w", err)
	}

	if !networkExists {
		fmt.Printf("Creating network %s...\n", networkName)
		if err := p.Runtime.CreateNetwork(ctx, networkName); err != nil {
			return fmt.Errorf("failed to create network: %w", err)
		}
	}

	// Create project volumes if they don't exist
	for volumeName := range p.Config.Volumes {
		fullName := p.VolumeName(volumeName)
		exists, err := p.Runtime.VolumeExists(ctx, fullName)
		if err != nil {
			return fmt.Errorf("failed to check volume %s: %w", volumeName, err)
		}
		if !exists {
			fmt.Printf("Creating volume %s...\n", fullName)
			if err := p.Runtime.CreateVolume(ctx, fullName); err != nil {
				return fmt.Errorf("failed to create volume %s: %w", volumeName, err)
			}
		}
	}

	// Build map of Mutagen mounts for quick lookup
	mutagenMountMap := make(map[string]MutagenSyncMount)
	for _, mount := range mutagenMounts {
		mutagenMountMap[mount.ServiceName] = mount
	}

	// Start all services
	for serviceName, serviceCfg := range p.Config.Services {
		if err := p.startServiceWithMutagen(ctx, serviceName, serviceCfg, mutagenEnabled, mutagenMountMap); err != nil {
			return fmt.Errorf("failed to start service %s: %w", serviceName, err)
		}
	}

	// Start Mutagen sync sessions after containers are running
	if mutagenEnabled && len(mutagenMounts) > 0 {
		if err := p.startMutagenSessions(ctx, m, mutagenMounts); err != nil {
			return fmt.Errorf("failed to start Mutagen sync: %w", err)
		}

		// Wait for initial sync (60 second timeout)
		p.waitForInitialSync(ctx, m, mutagenMounts, 60*time.Second)
	}

	// Register project with routing info in state
	tcpPorts, udpPorts := p.GetRequiredPorts()
	stateMgr, err := state.DefaultManager()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	if err := stateMgr.RegisterProjectWithRouting(p.Config.Name, p.Dir, tcpPorts, udpPorts); err != nil {
		return fmt.Errorf("failed to register project: %w", err)
	}

	// Connect router if enabled
	if p.Config.Shared.Router {
		if err := p.connectRouter(ctx); err != nil {
			fmt.Printf("Warning: could not connect router: %v\n", err)
		}
	}

	// Connect mail if enabled
	if p.Config.Shared.Mail {
		if err := p.connectMail(ctx); err != nil {
			fmt.Printf("Warning: could not connect mail: %v\n", err)
		}
	}

	// Connect db UI if enabled
	if p.Config.Shared.DBUI {
		if err := p.connectDBUI(ctx); err != nil {
			fmt.Printf("Warning: could not connect db: %v\n", err)
		}
	}

	// Connect redis insights if enabled
	if p.Config.Shared.RedisInsights {
		if err := p.connectRedisInsights(ctx); err != nil {
			fmt.Printf("Warning: could not connect redis insights: %v\n", err)
		}
	}

	return nil
}

func (p *Project) startService(ctx context.Context, name string, svc config.ServiceConfig) error {
	return p.startServiceWithMutagen(ctx, name, svc, false, nil)
}

// startServiceWithMutagen starts a service with optional Mutagen volume transformation
func (p *Project) startServiceWithMutagen(ctx context.Context, name string, svc config.ServiceConfig, mutagenEnabled bool, mutagenMounts map[string]MutagenSyncMount) error {
	containerName := p.ContainerName(name)

	// Check if container already exists
	exists, err := p.Runtime.ContainerExists(ctx, containerName)
	if err != nil {
		return err
	}

	if exists {
		running, err := p.Runtime.IsContainerRunning(ctx, containerName)
		if err != nil {
			return err
		}

		if running {
			fmt.Printf("Service %s is already running\n", name)
			return nil
		}

		fmt.Printf("Starting service %s...\n", name)
		return p.Runtime.StartContainer(ctx, containerName)
	}

	// Pull image if needed
	imageExists, err := p.Runtime.ImageExists(ctx, svc.Image)
	if err != nil {
		return err
	}

	if !imageExists {
		fmt.Printf("Pulling image %s...\n", svc.Image)
		if err := p.Runtime.PullImage(ctx, svc.Image); err != nil {
			return err
		}
	}

	// Build container config
	cfg := runtime.ContainerConfig{
		Name:        containerName,
		Image:       svc.Image,
		WorkingDir:  svc.WorkingDir,
		NetworkName: p.NetworkName(),
		Aliases:     []string{name}, // Service name as DNS alias
		Labels: map[string]string{
			"scdev.managed": "true",
			"scdev.project": p.Config.Name,
			"scdev.service": name,
		},
	}

	// Merge global and service environment
	cfg.Env = make(map[string]string)
	for k, v := range p.Config.Environment {
		cfg.Env[k] = v
	}
	for k, v := range svc.Environment {
		cfg.Env[k] = v
	}

	// Add USER_ID and GROUP_ID for bind mount permission handling
	if _, exists := cfg.Env["USER_ID"]; !exists {
		cfg.Env["USER_ID"] = fmt.Sprintf("%d", os.Getuid())
	}
	if _, exists := cfg.Env["GROUP_ID"]; !exists {
		cfg.Env["GROUP_ID"] = fmt.Sprintf("%d", os.Getgid())
	}

	// Add service labels (for Traefik, etc.)
	for k, v := range svc.Labels {
		cfg.Labels[k] = v
	}

	// Configure Traefik routing if specified and project uses shared router
	if svc.Routing != nil && p.Config.Shared.Router {
		p.configureRouting(&cfg, name, svc.Routing, isTLSAvailable())
	}

	// Parse and add volume mounts, transforming for Mutagen if enabled
	if mutagenEnabled && mutagenMounts != nil {
		cfg.Volumes = p.transformVolumesForMutagen(name, svc.Volumes, mutagenMounts)
	} else {
		for _, vol := range svc.Volumes {
			source, target, isNamedVolume := parseVolumeMount(vol)

			// For named volumes, prefix with project name
			if isNamedVolume {
				source = p.VolumeName(source)
			}

			cfg.Volumes = append(cfg.Volumes, runtime.VolumeMount{
				Source: source,
				Target: target,
			})
		}
	}

	// Parse command if specified
	if svc.Command != "" {
		cfg.Command = []string{"sh", "-c", svc.Command}
	}

	// Create and start
	fmt.Printf("Creating service %s...\n", name)
	if _, err := p.Runtime.CreateContainer(ctx, cfg); err != nil {
		return err
	}

	fmt.Printf("Starting service %s...\n", name)
	return p.Runtime.StartContainer(ctx, containerName)
}

// Stop stops all project services
func (p *Project) Stop(ctx context.Context) error {
	// Pause Mutagen sync sessions first (before stopping containers)
	if p.IsMutagenEnabled() {
		p.pauseMutagenSessions(ctx)
	}

	for serviceName := range p.Config.Services {
		containerName := p.ContainerName(serviceName)

		exists, err := p.Runtime.ContainerExists(ctx, containerName)
		if err != nil {
			return err
		}

		if !exists {
			continue
		}

		running, err := p.Runtime.IsContainerRunning(ctx, containerName)
		if err != nil {
			return err
		}

		if !running {
			fmt.Printf("Service %s is not running\n", serviceName)
			continue
		}

		fmt.Printf("Stopping service %s...\n", serviceName)
		if err := p.Runtime.StopContainer(ctx, containerName); err != nil {
			return fmt.Errorf("failed to stop service %s: %w", serviceName, err)
		}
	}
	return nil
}

// Down stops and removes all project containers and the network
// If removeVolumes is true, also removes volumes (respecting persist_on_delete)
func (p *Project) Down(ctx context.Context, removeVolumes bool) error {
	// Terminate Mutagen sync sessions first (before removing containers)
	if p.IsMutagenEnabled() {
		p.terminateMutagenSessions(ctx)
	}

	// Disconnect shared services (do this first, before removing network)
	if p.Config.Shared.RedisInsights {
		p.disconnectRedisInsights(ctx) // Ignore errors
	}
	if p.Config.Shared.DBUI {
		p.disconnectDBUI(ctx) // Ignore errors
	}
	if p.Config.Shared.Mail {
		p.disconnectMail(ctx) // Ignore errors
	}
	if p.Config.Shared.Router {
		p.disconnectRouter(ctx) // Ignore errors
	}

	// Remove all containers first
	for serviceName := range p.Config.Services {
		containerName := p.ContainerName(serviceName)

		exists, err := p.Runtime.ContainerExists(ctx, containerName)
		if err != nil {
			return err
		}

		if !exists {
			continue
		}

		// Stop if running
		running, err := p.Runtime.IsContainerRunning(ctx, containerName)
		if err != nil {
			return err
		}

		if running {
			fmt.Printf("Stopping service %s...\n", serviceName)
			if err := p.Runtime.StopContainer(ctx, containerName); err != nil {
				return fmt.Errorf("failed to stop service %s: %w", serviceName, err)
			}
		}

		// Remove
		fmt.Printf("Removing service %s...\n", serviceName)
		if err := p.Runtime.RemoveContainer(ctx, containerName); err != nil {
			return fmt.Errorf("failed to remove service %s: %w", serviceName, err)
		}
	}

	// Remove network
	networkName := p.NetworkName()
	networkExists, err := p.Runtime.NetworkExists(ctx, networkName)
	if err != nil {
		return fmt.Errorf("failed to check network: %w", err)
	}

	if networkExists {
		fmt.Printf("Removing network %s...\n", networkName)
		if err := p.Runtime.RemoveNetwork(ctx, networkName); err != nil {
			return fmt.Errorf("failed to remove network: %w", err)
		}
	}

	// Remove volumes if requested
	if removeVolumes {
		// Remove project volumes
		for volumeName := range p.Config.Volumes {
			fullName := p.VolumeName(volumeName)
			exists, err := p.Runtime.VolumeExists(ctx, fullName)
			if err != nil {
				return fmt.Errorf("failed to check volume %s: %w", volumeName, err)
			}

			if exists {
				fmt.Printf("Removing volume %s...\n", fullName)
				if err := p.Runtime.RemoveVolume(ctx, fullName); err != nil {
					return fmt.Errorf("failed to remove volume %s: %w", volumeName, err)
				}
			}
		}

		// Remove Mutagen sync volumes
		if p.IsMutagenEnabled() {
			p.removeMutagenVolumes(ctx)
		}
	}

	return nil
}

// Update checks for config changes and recreates containers as needed
// Returns true if any changes were made
func (p *Project) Update(ctx context.Context) (bool, error) {
	// Check port availability for new ports
	if err := p.checkPortAvailability(ctx); err != nil {
		return false, err
	}

	// Ensure network exists
	networkName := p.NetworkName()
	networkExists, err := p.Runtime.NetworkExists(ctx, networkName)
	if err != nil {
		return false, fmt.Errorf("failed to check network: %w", err)
	}

	if !networkExists {
		// Project not started yet, just run Start
		return true, p.Start(ctx)
	}

	// Ensure volumes exist
	for volumeName := range p.Config.Volumes {
		fullName := p.VolumeName(volumeName)
		exists, err := p.Runtime.VolumeExists(ctx, fullName)
		if err != nil {
			return false, fmt.Errorf("failed to check volume %s: %w", volumeName, err)
		}
		if !exists {
			fmt.Printf("Creating volume %s...\n", fullName)
			if err := p.Runtime.CreateVolume(ctx, fullName); err != nil {
				return false, fmt.Errorf("failed to create volume %s: %w", volumeName, err)
			}
		}
	}

	updated := false

	// Check each service for changes
	for serviceName, svc := range p.Config.Services {
		containerName := p.ContainerName(serviceName)

		exists, err := p.Runtime.ContainerExists(ctx, containerName)
		if err != nil {
			return false, err
		}

		if !exists {
			// Container doesn't exist, create it
			fmt.Printf("Creating service %s...\n", serviceName)
			if err := p.startService(ctx, serviceName, svc); err != nil {
				return false, fmt.Errorf("failed to start service %s: %w", serviceName, err)
			}
			updated = true
			continue
		}

		// Check if container needs recreation
		needsRecreate, err := p.serviceNeedsRecreate(ctx, serviceName, svc)
		if err != nil {
			return false, err
		}

		if needsRecreate {
			fmt.Printf("Recreating service %s...\n", serviceName)

			// Stop and remove old container
			running, _ := p.Runtime.IsContainerRunning(ctx, containerName)
			if running {
				if err := p.Runtime.StopContainer(ctx, containerName); err != nil {
					return false, fmt.Errorf("failed to stop service %s: %w", serviceName, err)
				}
			}
			if err := p.Runtime.RemoveContainer(ctx, containerName); err != nil {
				return false, fmt.Errorf("failed to remove service %s: %w", serviceName, err)
			}

			// Create new container
			if err := p.startService(ctx, serviceName, svc); err != nil {
				return false, fmt.Errorf("failed to start service %s: %w", serviceName, err)
			}
			updated = true
		} else {
			// Ensure container is running
			running, _ := p.Runtime.IsContainerRunning(ctx, containerName)
			if !running {
				fmt.Printf("Starting service %s...\n", serviceName)
				if err := p.Runtime.StartContainer(ctx, containerName); err != nil {
					return false, fmt.Errorf("failed to start service %s: %w", serviceName, err)
				}
			}
		}
	}

	// Update state with current routing info
	tcpPorts, udpPorts := p.GetRequiredPorts()
	stateMgr, err := state.DefaultManager()
	if err != nil {
		return updated, fmt.Errorf("failed to load state: %w", err)
	}
	if err := stateMgr.RegisterProjectWithRouting(p.Config.Name, p.Dir, tcpPorts, udpPorts); err != nil {
		return updated, fmt.Errorf("failed to register project: %w", err)
	}

	// Update router if using shared router
	if p.Config.Shared.Router {
		if err := p.connectRouter(ctx); err != nil {
			fmt.Printf("Warning: could not connect router: %v\n", err)
		}
	}

	// Update mail if using shared mail
	if p.Config.Shared.Mail {
		if err := p.connectMail(ctx); err != nil {
			fmt.Printf("Warning: could not connect mail: %v\n", err)
		}
	}

	// Update db UI if using shared db
	if p.Config.Shared.DBUI {
		if err := p.connectDBUI(ctx); err != nil {
			fmt.Printf("Warning: could not connect db: %v\n", err)
		}
	}

	// Update redis insights if using shared redis insights
	if p.Config.Shared.RedisInsights {
		if err := p.connectRedisInsights(ctx); err != nil {
			fmt.Printf("Warning: could not connect redis insights: %v\n", err)
		}
	}

	return updated, nil
}

// serviceNeedsRecreate checks if a service container needs to be recreated
func (p *Project) serviceNeedsRecreate(ctx context.Context, serviceName string, svc config.ServiceConfig) (bool, error) {
	containerName := p.ContainerName(serviceName)

	// Get current container labels
	currentLabels, err := p.Runtime.GetContainerLabels(ctx, containerName)
	if err != nil {
		return true, nil // If we can't read labels, recreate to be safe
	}

	// Build expected labels for comparison
	expectedCfg := p.buildContainerConfig(serviceName, svc)

	// Compare routing-related labels
	for key, expectedValue := range expectedCfg.Labels {
		if strings.HasPrefix(key, "traefik.") {
			if currentLabels[key] != expectedValue {
				return true, nil
			}
		}
	}

	// Check if any traefik labels in current config are not in expected (removed routing)
	for key := range currentLabels {
		if strings.HasPrefix(key, "traefik.") {
			if _, ok := expectedCfg.Labels[key]; !ok {
				return true, nil
			}
		}
	}

	// TODO: Could also compare image, env, volumes, etc.
	// For now, focus on routing changes

	return false, nil
}

// buildContainerConfig builds the container config for a service (without creating it)
func (p *Project) buildContainerConfig(name string, svc config.ServiceConfig) runtime.ContainerConfig {
	containerName := p.ContainerName(name)

	cfg := runtime.ContainerConfig{
		Name:        containerName,
		Image:       svc.Image,
		NetworkName: p.NetworkName(),
		Aliases:     []string{name},
		Env:         make(map[string]string),
		Labels: map[string]string{
			"scdev.managed": "true",
			"scdev.project": p.Config.Name,
			"scdev.service": name,
		},
	}

	// Merge global environment first
	for k, v := range p.Config.Environment {
		cfg.Env[k] = v
	}

	// Then service-specific environment (overrides global)
	for k, v := range svc.Environment {
		cfg.Env[k] = v
	}

	// Add USER_ID and GROUP_ID for bind mount permission handling
	// These can be overridden by project/service config if needed
	if _, exists := cfg.Env["USER_ID"]; !exists {
		cfg.Env["USER_ID"] = fmt.Sprintf("%d", os.Getuid())
	}
	if _, exists := cfg.Env["GROUP_ID"]; !exists {
		cfg.Env["GROUP_ID"] = fmt.Sprintf("%d", os.Getgid())
	}

	// Add working directory
	if svc.WorkingDir != "" {
		cfg.WorkingDir = svc.WorkingDir
	}

	// Configure routing if specified
	if svc.Routing != nil && p.Config.Shared.Router {
		p.configureRouting(&cfg, name, svc.Routing, isTLSAvailable())
	}

	// Add any explicit labels from config
	for k, v := range svc.Labels {
		cfg.Labels[k] = v
	}

	// Parse and add volume mounts
	for _, vol := range svc.Volumes {
		source, target, isNamedVolume := parseVolumeMount(vol)
		if isNamedVolume {
			source = p.VolumeName(source)
		}
		cfg.Volumes = append(cfg.Volumes, runtime.VolumeMount{
			Source: source,
			Target: target,
		})
	}

	// Parse command
	if svc.Command != "" {
		cfg.Command = []string{"sh", "-c", svc.Command}
	}

	return cfg
}

// Exec runs a command in a service container
func (p *Project) Exec(ctx context.Context, service string, command []string, interactive bool, opts ExecOptions) error {
	// Verify service exists
	if _, ok := p.Config.Services[service]; !ok {
		return fmt.Errorf("unknown service: %s", service)
	}

	containerName := p.ContainerName(service)

	// Check if running
	running, err := p.Runtime.IsContainerRunning(ctx, containerName)
	if err != nil {
		return err
	}

	if !running {
		return fmt.Errorf("service %s is not running", service)
	}

	runtimeOpts := runtime.ExecOptions{
		User:    opts.User,
		Workdir: opts.Workdir,
	}

	return p.Runtime.Exec(ctx, containerName, command, interactive, runtimeOpts)
}

// LogsOptions configures log output behavior
type LogsOptions struct {
	Follow bool // Stream logs in real-time
	Tail   int  // Number of lines to show from end (0 = all)
}

// Logs streams logs from a service container
func (p *Project) Logs(ctx context.Context, service string, opts LogsOptions) error {
	// Verify service exists
	if _, ok := p.Config.Services[service]; !ok {
		return fmt.Errorf("unknown service: %s", service)
	}

	containerName := p.ContainerName(service)

	// Check if container exists
	exists, err := p.Runtime.ContainerExists(ctx, containerName)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("service %s container does not exist - run 'scdev start' first", service)
	}

	runtimeOpts := runtime.LogsOptions{
		Follow: opts.Follow,
		Tail:   opts.Tail,
	}

	return p.Runtime.Logs(ctx, containerName, runtimeOpts)
}

// Restart stops and starts the project
func (p *Project) Restart(ctx context.Context) error {
	if err := p.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop: %w", err)
	}

	if err := p.Start(ctx); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	return nil
}

// ServiceNames returns a list of all service names
func (p *Project) ServiceNames() []string {
	names := make([]string, 0, len(p.Config.Services))
	for name := range p.Config.Services {
		names = append(names, name)
	}
	return names
}

// VolumeInfo contains information about a project volume
type VolumeInfo struct {
	Name            string
	FullName        string
	Exists   bool
}

// Volumes returns information about all project volumes
func (p *Project) Volumes(ctx context.Context) ([]VolumeInfo, error) {
	var volumes []VolumeInfo

	for volumeName := range p.Config.Volumes {
		fullName := p.VolumeName(volumeName)
		exists, err := p.Runtime.VolumeExists(ctx, fullName)
		if err != nil {
			return nil, fmt.Errorf("failed to check volume %s: %w", volumeName, err)
		}

		volumes = append(volumes, VolumeInfo{
			Name:     volumeName,
			FullName: fullName,
			Exists:   exists,
		})
	}

	return volumes, nil
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

	// Router is special - needs ports from project
	tcpPorts, udpPorts := p.GetRequiredPorts()

	fmt.Println("Ensuring shared router is running...")
	if err := mgr.StartRouterWithPorts(ctx, tcpPorts, udpPorts); err != nil {
		return fmt.Errorf("failed to start router: %w", err)
	}

	return mgr.ConnectToProject(ctx, p.NetworkName())
}

// disconnectRouter disconnects the shared router from this project's network
func (p *Project) disconnectRouter(ctx context.Context) {
	p.disconnectSharedService(ctx, func(ctx context.Context, mgr *services.Manager, network string) error {
		return mgr.DisconnectFromProject(ctx, network)
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

// ============================================================================
// Mutagen file sync integration
// ============================================================================

// MutagenSyncMount describes a bind mount to be synced via Mutagen
type MutagenSyncMount struct {
	ServiceName   string // Service this mount belongs to
	HostPath      string // Absolute path on host
	ContainerPath string // Path inside container
	VolumeName    string // Docker volume name for sync
	SessionName   string // Mutagen session name
}

// MutagenSessionName returns the Mutagen session name for a service
// Pattern: scdev-<project>-<service> (hyphens - Mutagen only allows alphanumeric and hyphens)
func (p *Project) MutagenSessionName(serviceName string) string {
	return fmt.Sprintf("scdev-%s-%s", p.Config.Name, serviceName)
}

// MutagenVolumeName returns the Docker volume name for Mutagen sync
// Same as session name for clarity
func (p *Project) MutagenVolumeName(serviceName string) string {
	return fmt.Sprintf("sync.%s.%s.scdev", serviceName, p.Config.Name)
}

// isBindMount checks if a volume string represents a bind mount (vs named volume)
func isBindMount(volume string) bool {
	source, _, isNamed := parseVolumeMount(volume)
	if isNamed {
		return false
	}
	// It's a bind mount if it starts with / or . or contains path separators
	return strings.HasPrefix(source, "/") || strings.HasPrefix(source, ".") || strings.Contains(source, string(os.PathSeparator))
}

// GetMutagenSyncMounts returns all bind mounts that should be synced via Mutagen
// Only directories are synced - file mounts are kept as regular bind mounts
func (p *Project) GetMutagenSyncMounts() []MutagenSyncMount {
	var mounts []MutagenSyncMount

	for serviceName, svc := range p.Config.Services {
		for _, vol := range svc.Volumes {
			if !isBindMount(vol) {
				continue
			}

			source, target, _ := parseVolumeMount(vol)

			// Resolve source to absolute path
			absSource := source
			if !filepath.IsAbs(source) {
				absSource = filepath.Join(p.Dir, source)
			}

			// Only sync directories - Mutagen doesn't support single file sync
			info, err := os.Stat(absSource)
			if err != nil || !info.IsDir() {
				continue
			}

			mounts = append(mounts, MutagenSyncMount{
				ServiceName:   serviceName,
				HostPath:      absSource,
				ContainerPath: target,
				VolumeName:    p.MutagenVolumeName(serviceName),
				SessionName:   p.MutagenSessionName(serviceName),
			})
		}
	}

	return mounts
}

// EnsureMutagen ensures the Mutagen binary is available and daemon is running
func (p *Project) EnsureMutagen(ctx context.Context) (*mutagen.Mutagen, error) {
	toolMgr, err := tools.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tool manager: %w", err)
	}

	mutagenPath, err := toolMgr.EnsureTool(ctx, tools.MutagenTool())
	if err != nil {
		return nil, fmt.Errorf("failed to ensure mutagen: %w", err)
	}

	m := mutagen.New(mutagenPath)

	if err := m.EnsureDaemon(ctx); err != nil {
		return nil, fmt.Errorf("failed to start mutagen daemon: %w", err)
	}

	return m, nil
}

// createMutagenVolumes creates Docker volumes for Mutagen sync
func (p *Project) createMutagenVolumes(ctx context.Context, mounts []MutagenSyncMount) error {
	for _, mount := range mounts {
		exists, err := p.Runtime.VolumeExists(ctx, mount.VolumeName)
		if err != nil {
			return fmt.Errorf("failed to check volume %s: %w", mount.VolumeName, err)
		}
		if !exists {
			fmt.Printf("Creating sync volume %s...\n", mount.VolumeName)
			if err := p.Runtime.CreateVolume(ctx, mount.VolumeName); err != nil {
				return fmt.Errorf("failed to create volume %s: %w", mount.VolumeName, err)
			}
		}
	}
	return nil
}

// startMutagenSessions creates or resumes Mutagen sync sessions
func (p *Project) startMutagenSessions(ctx context.Context, m *mutagen.Mutagen, mounts []MutagenSyncMount) error {
	globalCfg, _ := config.LoadGlobalConfig()

	for _, mount := range mounts {
		exists, err := m.SessionExists(ctx, mount.SessionName)
		if err != nil {
			return fmt.Errorf("failed to check session %s: %w", mount.SessionName, err)
		}

		containerName := p.ContainerName(mount.ServiceName)
		beta := fmt.Sprintf("docker://%s%s", containerName, mount.ContainerPath)

		if exists {
			// Resume existing session
			fmt.Printf("Resuming sync session %s...\n", mount.SessionName)
			if err := m.ResumeSession(ctx, mount.SessionName); err != nil {
				// Ignore resume errors - session might already be running
				fmt.Printf("Note: could not resume session (may already be running): %v\n", err)
			}
		} else {
			// Create new session
			fmt.Printf("Creating sync session %s...\n", mount.SessionName)

			// Collect ignores from project config and built-in defaults
			ignores := mutagen.MergeIgnores(p.Config.Mutagen.Ignore)

			cfg := mutagen.SessionConfig{
				Name:    mount.SessionName,
				Alpha:   mount.HostPath,
				Beta:    beta,
				Ignores: ignores,
			}

			// Use sync mode from global config if set
			if globalCfg != nil && globalCfg.Mutagen.SyncMode != "" {
				// SessionConfig doesn't have SyncMode yet, it's hardcoded in CreateSession
				// TODO: Add SyncMode to SessionConfig
			}

			if err := m.CreateSession(ctx, cfg); err != nil {
				return fmt.Errorf("failed to create session %s: %w", mount.SessionName, err)
			}
		}
	}

	return nil
}

// pauseMutagenSessions pauses all Mutagen sync sessions for this project
func (p *Project) pauseMutagenSessions(ctx context.Context) {
	m, err := p.EnsureMutagen(ctx)
	if err != nil {
		return // Silently ignore - Mutagen might not be set up
	}

	mounts := p.GetMutagenSyncMounts()
	for _, mount := range mounts {
		exists, _ := m.SessionExists(ctx, mount.SessionName)
		if exists {
			fmt.Printf("Pausing sync session %s...\n", mount.SessionName)
			if err := m.PauseSession(ctx, mount.SessionName); err != nil {
				fmt.Printf("Warning: could not pause session %s: %v\n", mount.SessionName, err)
			}
		}
	}
}

// terminateMutagenSessions terminates all Mutagen sync sessions for this project
func (p *Project) terminateMutagenSessions(ctx context.Context) {
	m, err := p.EnsureMutagen(ctx)
	if err != nil {
		return // Silently ignore
	}

	mounts := p.GetMutagenSyncMounts()
	for _, mount := range mounts {
		exists, _ := m.SessionExists(ctx, mount.SessionName)
		if exists {
			fmt.Printf("Terminating sync session %s...\n", mount.SessionName)
			if err := m.TerminateSession(ctx, mount.SessionName); err != nil {
				fmt.Printf("Warning: could not terminate session %s: %v\n", mount.SessionName, err)
			}
		}
	}
}

// removeMutagenVolumes removes Mutagen sync volumes
func (p *Project) removeMutagenVolumes(ctx context.Context) {
	mounts := p.GetMutagenSyncMounts()
	for _, mount := range mounts {
		exists, _ := p.Runtime.VolumeExists(ctx, mount.VolumeName)
		if exists {
			fmt.Printf("Removing sync volume %s...\n", mount.VolumeName)
			if err := p.Runtime.RemoveVolume(ctx, mount.VolumeName); err != nil {
				fmt.Printf("Warning: could not remove volume %s: %v\n", mount.VolumeName, err)
			}
		}
	}
}

// waitForInitialSync waits for Mutagen sync sessions to complete initial sync
func (p *Project) waitForInitialSync(ctx context.Context, m *mutagen.Mutagen, mounts []MutagenSyncMount, timeout time.Duration) {
	if len(mounts) == 0 {
		return
	}

	fmt.Println("Waiting for initial file sync...")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for _, mount := range mounts {
		if err := m.FlushSession(ctx, mount.SessionName); err != nil {
			if ctx.Err() != nil {
				fmt.Printf("Warning: sync timeout - files may still be syncing in the background\n")
				return
			}
			fmt.Printf("Warning: could not wait for sync %s: %v\n", mount.SessionName, err)
		}
	}

	fmt.Println("Initial sync complete")
}

// IsMutagenEnabled checks if Mutagen is enabled for this project
func (p *Project) IsMutagenEnabled() bool {
	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return false
	}
	return globalCfg.IsMutagenEnabled()
}

// transformVolumesForMutagen transforms bind mounts to Mutagen sync volumes
// Returns the modified volumes list for container creation
func (p *Project) transformVolumesForMutagen(serviceName string, volumes []string, mutagenMounts map[string]MutagenSyncMount) []runtime.VolumeMount {
	var result []runtime.VolumeMount

	for _, vol := range volumes {
		source, target, isNamedVolume := parseVolumeMount(vol)

		if isNamedVolume {
			// Named volume - prefix with project name
			result = append(result, runtime.VolumeMount{
				Source: p.VolumeName(source),
				Target: target,
			})
		} else if isBindMount(vol) {
			// Bind mount - use Mutagen sync volume instead
			mount, ok := mutagenMounts[serviceName]
			if ok && mount.ContainerPath == target {
				result = append(result, runtime.VolumeMount{
					Source: mount.VolumeName,
					Target: target,
				})
			} else {
				// Fallback to bind mount if not in Mutagen mounts
				result = append(result, runtime.VolumeMount{
					Source: source,
					Target: target,
				})
			}
		} else {
			// Regular bind mount
			result = append(result, runtime.VolumeMount{
				Source: source,
				Target: target,
			})
		}
	}

	return result
}
