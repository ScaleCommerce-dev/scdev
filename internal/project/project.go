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

// ContainerNameFor returns the full container name for a service in a given project.
// Format: <service>.<project>.scdev (e.g., app.myproject.scdev)
// This standalone function can be used without a loaded Project.
func ContainerNameFor(service, projectName string) string {
	return fmt.Sprintf("%s.%s.scdev", service, projectName)
}

// ContainerName returns the full container name for a service
// Format: <service>.<project>.scdev (e.g., app.myproject.scdev)
func (p *Project) ContainerName(service string) string {
	return ContainerNameFor(service, p.Config.Name)
}

// NetworkName returns the project network name
// Format: <project>.scdev (e.g., myproject.scdev)
func (p *Project) NetworkName() string {
	return fmt.Sprintf("%s.scdev", p.Config.Name)
}

// shellQuote wraps a string in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// VolumeName returns the full volume name for a project volume
// Format: <volume>.<project>.scdev (e.g., db_data.myproject.scdev)
func (p *Project) VolumeName(volume string) string {
	return fmt.Sprintf("%s.%s.scdev", volume, p.Config.Name)
}

// NamedVolumes returns all named volumes discovered from service volume mounts.
func (p *Project) NamedVolumes() []string {
	seen := make(map[string]bool)
	var names []string
	for _, svc := range p.Config.Services {
		for _, vol := range svc.Volumes {
			source, _, isNamed := parseVolumeMount(vol)
			if isNamed && !seen[source] {
				seen[source] = true
				names = append(names, source)
			}
		}
	}
	return names
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
	for _, volumeName := range p.NamedVolumes() {
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

		// Signal containers that sync is ready (unblocks the sync-ready gate)
		p.signalSyncReady(ctx, mutagenMounts)
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

	// Connect shared services
	p.connectEnabledSharedServices(ctx)

	// Connect to link networks
	p.connectLinks(ctx)

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

	// Build container config (single source of truth)
	cfg := p.buildContainerConfig(name, svc, mutagenEnabled, mutagenMounts)

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

	// Disconnect from link networks (before removing containers)
	p.disconnectLinks(ctx)

	// Disconnect shared services (do this first, before removing network)
	p.disconnectEnabledSharedServices(ctx)

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
		for _, volumeName := range p.NamedVolumes() {
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

	// Unregister from global state
	stateMgr, err := state.DefaultManager()
	if err == nil {
		_ = stateMgr.UnregisterProject(p.Config.Name)
	}

	// Refresh router to release any TCP/UDP ports this project was using
	if p.Config.Shared.Router {
		globalCfg, err := config.LoadGlobalConfig()
		if err == nil {
			mgr := services.NewManager(globalCfg)
			_ = mgr.RefreshRouter(ctx)
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
	for _, volumeName := range p.NamedVolumes() {
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

	// Connect shared services
	p.connectEnabledSharedServices(ctx)

	// Connect to link networks
	p.connectLinks(ctx)

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

	// Build expected config using the same logic as Start() for accurate comparison
	mutagenEnabled := p.IsMutagenEnabled()
	var mutagenMountMap map[string]MutagenSyncMount
	if mutagenEnabled {
		mutagenMountMap = make(map[string]MutagenSyncMount)
		for _, mount := range p.GetMutagenSyncMounts() {
			mutagenMountMap[mount.ServiceName] = mount
		}
	}
	expectedCfg := p.buildContainerConfig(serviceName, svc, mutagenEnabled, mutagenMountMap)

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

	// For now, focus on routing changes

	return false, nil
}

// buildContainerConfig builds the full container configuration for a service.
// This is the single source of truth for container config - used by both
// startServiceWithMutagen (for creating containers) and serviceNeedsRecreate
// (for comparing against running containers).
func (p *Project) buildContainerConfig(name string, svc config.ServiceConfig, mutagenEnabled bool, mutagenMounts map[string]MutagenSyncMount) runtime.ContainerConfig {
	containerName := p.ContainerName(name)

	cfg := runtime.ContainerConfig{
		Name:        containerName,
		Image:       svc.Image,
		WorkingDir:  svc.WorkingDir,
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
	if _, exists := cfg.Env["USER_ID"]; !exists {
		cfg.Env["USER_ID"] = fmt.Sprintf("%d", os.Getuid())
	}
	if _, exists := cfg.Env["GROUP_ID"]; !exists {
		cfg.Env["GROUP_ID"] = fmt.Sprintf("%d", os.Getgid())
	}

	// Add any explicit labels from config (before routing, so routing labels take precedence)
	for k, v := range svc.Labels {
		cfg.Labels[k] = v
	}

	// Configure routing if specified (after user labels, so routing wins on conflict)
	if svc.Routing != nil && p.Config.Shared.Router {
		p.configureRouting(&cfg, name, svc.Routing, isTLSAvailable())
	}

	// Parse and add volume mounts, transforming for Mutagen if enabled
	if mutagenEnabled && mutagenMounts != nil {
		cfg.Volumes = p.transformVolumesForMutagen(name, svc.Volumes, mutagenMounts)
	} else {
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
	}

	// Parse command
	if svc.Command != "" {
		// When Mutagen is enabled for this service, wrap with sync-ready gate
		_, hasMutagenMount := mutagenMounts[name]
		if mutagenEnabled && hasMutagenMount {
			cfg.Command = []string{"sh", "-c",
				"while [ ! -f /.scdev-sync-ready ]; do sleep 0.2; done; exec sh -c " + shellQuote(svc.Command),
			}
		} else {
			cfg.Command = []string{"sh", "-c", svc.Command}
		}
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

	for _, volumeName := range p.NamedVolumes() {
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

// sharedServiceEntry pairs a config flag with its connect/disconnect methods.
