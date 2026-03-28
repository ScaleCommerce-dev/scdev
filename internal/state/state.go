package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// RoutingState tracks the routing ports used by a project
type RoutingState struct {
	TCPPorts []int `yaml:"tcp_ports,omitempty"`
	UDPPorts []int `yaml:"udp_ports,omitempty"`
}

// ProjectEntry represents a registered project in the state file
type ProjectEntry struct {
	Path        string       `yaml:"path"`
	LastStarted time.Time    `yaml:"last_started,omitempty"`
	Routing     RoutingState `yaml:"routing,omitempty"`
}

// State represents the global scdev state
type State struct {
	Projects map[string]ProjectEntry `yaml:"projects"`
}

// Manager handles reading and writing the state file
type Manager struct {
	path string
	mu   sync.Mutex
}

// DefaultManager returns a manager using the default state file location
func DefaultManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	scdevDir := filepath.Join(homeDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .scdev directory: %w", err)
	}

	return &Manager{
		path: filepath.Join(scdevDir, "state.yaml"),
	}, nil
}

// NewManager creates a state manager with a custom path (useful for testing)
func NewManager(path string) *Manager {
	return &Manager{path: path}
}

// Load reads the state file
func (m *Manager) Load() (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := &State{
		Projects: make(map[string]ProjectEntry),
	}

	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil // Return empty state if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	if err := yaml.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	// Ensure Projects map is initialized
	if state.Projects == nil {
		state.Projects = make(map[string]ProjectEntry)
	}

	return state, nil
}

// Save writes the state file
func (m *Manager) Save(state *State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// RegisterProject adds or updates a project in the state
func (m *Manager) RegisterProject(name, path string) error {
	return m.RegisterProjectWithRouting(name, path, nil, nil)
}

// RegisterProjectWithRouting adds or updates a project with routing info
func (m *Manager) RegisterProjectWithRouting(name, path string, tcpPorts, udpPorts []int) error {
	state, err := m.Load()
	if err != nil {
		return err
	}

	state.Projects[name] = ProjectEntry{
		Path:        path,
		LastStarted: time.Now(),
		Routing: RoutingState{
			TCPPorts: tcpPorts,
			UDPPorts: udpPorts,
		},
	}

	return m.Save(state)
}

// UnregisterProject removes a project from the state
func (m *Manager) UnregisterProject(name string) error {
	state, err := m.Load()
	if err != nil {
		return err
	}

	delete(state.Projects, name)

	return m.Save(state)
}

// GetProject returns a single project entry
func (m *Manager) GetProject(name string) (*ProjectEntry, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}

	entry, ok := state.Projects[name]
	if !ok {
		return nil, nil
	}

	return &entry, nil
}

// ListProjects returns all registered projects
func (m *Manager) ListProjects() (map[string]ProjectEntry, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}

	return state.Projects, nil
}

// GetAllRoutingPorts aggregates all TCP and UDP ports from all projects
func (m *Manager) GetAllRoutingPorts() (tcpPorts, udpPorts []int, err error) {
	state, err := m.Load()
	if err != nil {
		return nil, nil, err
	}

	tcpSet := make(map[int]bool)
	udpSet := make(map[int]bool)

	for _, entry := range state.Projects {
		for _, port := range entry.Routing.TCPPorts {
			tcpSet[port] = true
		}
		for _, port := range entry.Routing.UDPPorts {
			udpSet[port] = true
		}
	}

	for port := range tcpSet {
		tcpPorts = append(tcpPorts, port)
	}
	for port := range udpSet {
		udpPorts = append(udpPorts, port)
	}

	return tcpPorts, udpPorts, nil
}

// GetTCPPortOwner returns the project name that owns the given TCP port, or empty string
func (m *Manager) GetTCPPortOwner(port int) (string, error) {
	state, err := m.Load()
	if err != nil {
		return "", err
	}

	for name, entry := range state.Projects {
		for _, p := range entry.Routing.TCPPorts {
			if p == port {
				return name, nil
			}
		}
	}

	return "", nil
}

// GetUDPPortOwner returns the project name that owns the given UDP port, or empty string
func (m *Manager) GetUDPPortOwner(port int) (string, error) {
	state, err := m.Load()
	if err != nil {
		return "", err
	}

	for name, entry := range state.Projects {
		for _, p := range entry.Routing.UDPPorts {
			if p == port {
				return name, nil
			}
		}
	}

	return "", nil
}
