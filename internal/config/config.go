package config

import "runtime"

// GlobalConfig represents ~/.scdev/config.yaml
type GlobalConfig struct {
	Version  int                  `yaml:"version"`
	Domain   string               `yaml:"domain"`
	Runtime  string               `yaml:"runtime"`
	SSL      SSLConfig            `yaml:"ssl"`
	Shared   SharedConfig         `yaml:"shared"`
	Terminal TerminalConfig       `yaml:"terminal"`
	Mutagen  MutagenGlobalConfig  `yaml:"mutagen"`
}

// MutagenGlobalConfig defines global Mutagen file sync settings
type MutagenGlobalConfig struct {
	Enabled  string `yaml:"enabled"`   // "auto", "true", "false" - auto enables on macOS only
	SyncMode string `yaml:"sync_mode"` // Default sync mode (default: two-way-safe)
}

// IsMutagenEnabled returns whether Mutagen file sync should be used
// "auto" (default): enabled on macOS, disabled on Linux
// "true": always enabled
// "false": always disabled
func (g *GlobalConfig) IsMutagenEnabled() bool {
	switch g.Mutagen.Enabled {
	case "true":
		return true
	case "false":
		return false
	default: // "auto" or empty
		return runtime.GOOS == "darwin"
	}
}

// SSLConfig defines SSL/TLS certificate configuration
type SSLConfig struct {
	Enabled bool `yaml:"enabled"` // Enable HTTPS with mkcert certificates (default: true)
}

// TerminalConfig defines terminal output settings
type TerminalConfig struct {
	Plain bool `yaml:"plain"` // Disable colors, hyperlinks, and markdown rendering
}

// SharedConfig defines shared services configuration
type SharedConfig struct {
	Router        RouterConfig        `yaml:"router"`
	Mail          MailConfig          `yaml:"mail"`
	DBUI          DBUIConfig          `yaml:"db"`
	RedisInsights RedisInsightsConfig `yaml:"redis_insights"`
	Observability ObservabilityConfig `yaml:"observability"`
}

// RouterConfig defines Traefik configuration
type RouterConfig struct {
	Image     string `yaml:"image"`
	Dashboard bool   `yaml:"dashboard"`
}

// MailConfig defines Mailpit configuration
type MailConfig struct {
	Image string `yaml:"image"`
}

// DBUIConfig defines Adminer configuration
type DBUIConfig struct {
	Image string `yaml:"image"`
}

// RedisInsightsConfig defines Redis Insights configuration
type RedisInsightsConfig struct {
	Image string `yaml:"image"`
}

// ObservabilityConfig defines OpenObserve configuration
type ObservabilityConfig struct {
	Image string `yaml:"image"`
}

// ProjectConfig represents .scdev/config.yaml
type ProjectConfig struct {
	Version         int                      `yaml:"version"`
	Name            string                   `yaml:"name"`
	Domain          string                   `yaml:"domain"`
	Info            string                   `yaml:"info"`
	AutoOpenAtStart bool                     `yaml:"auto_open_at_start"`
	Shared          ProjectSharedConfig      `yaml:"shared"`
	Environment     map[string]string        `yaml:"environment"`
	Services        map[string]ServiceConfig `yaml:"services"`
	Volumes         map[string]VolumeConfig  `yaml:"volumes"`
	Mutagen         ProjectMutagenConfig     `yaml:"mutagen"`
}

// ProjectMutagenConfig defines project-level Mutagen settings
type ProjectMutagenConfig struct {
	Ignore []string `yaml:"ignore"` // Paths to exclude from sync (not synced in either direction)
}

// ProjectSharedConfig defines which shared services a project uses
type ProjectSharedConfig struct {
	Router        bool `yaml:"router"`
	Mail          bool `yaml:"mail"`
	DBUI          bool `yaml:"db"`
	RedisInsights bool `yaml:"redis_insights"`
	Observability bool `yaml:"observability"`
	Tunnel        bool `yaml:"tunnel"`
}

// ServiceConfig defines a container service
type ServiceConfig struct {
	Image          string            `yaml:"image"`
	Routing        *RoutingConfig    `yaml:"routing"` // Traefik routing config (requires shared.router: true)
	WorkingDir     string            `yaml:"working_dir"`
	Volumes        []string          `yaml:"volumes"`
	Environment    map[string]string `yaml:"environment"`
	Command        string            `yaml:"command"`
	Labels         map[string]string `yaml:"labels"`
	PreStart       []string          `yaml:"pre_start"`
	RegisterToDBUI bool              `yaml:"register_to_dbui"` // Register this service in the shared DB UI (Adminer)
}

// RoutingConfig defines how a service is exposed via the shared router
type RoutingConfig struct {
	Protocol string `yaml:"protocol"`  // http, https, tcp, udp
	Port     int    `yaml:"port"`      // Container port (defaults: http=80, https=443, tcp/udp=required)
	HostPort int    `yaml:"host_port"` // Host port for tcp/udp (required for tcp/udp, ignored for http/https)
}

// VolumeConfig defines a named volume
type VolumeConfig struct {
	// Currently empty, but struct kept for future options (e.g., driver, labels)
}
