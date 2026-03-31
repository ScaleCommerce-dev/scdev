package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/mutagen"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/tools"
)

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

// signalSyncReady writes a marker file into each container that has a Mutagen sync mount,
// unblocking the sync-ready gate in the container's entrypoint wrapper.
func (p *Project) signalSyncReady(ctx context.Context, mounts []MutagenSyncMount) {
	for _, mount := range mounts {
		containerName := p.ContainerName(mount.ServiceName)
		err := p.Runtime.Exec(ctx, containerName,
			[]string{"sh", "-c", "touch /.scdev-sync-ready"}, false, runtime.ExecOptions{})
		if err != nil {
			fmt.Printf("Warning: could not signal sync-ready for %s: %v\n", mount.ServiceName, err)
		}
	}
}
