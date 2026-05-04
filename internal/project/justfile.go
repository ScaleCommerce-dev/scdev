package project

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/0ploy/zdev/internal/config"
)

// JustfileInfo represents a discovered justfile
type JustfileInfo struct {
	Name string // Command name (filename without .just)
	Path string // Full path to the justfile
}

// GetJustfile returns the justfile for a command name, if it exists
func (p *Project) GetJustfile(name string) (*JustfileInfo, error) {
	justfilePath := filepath.Join(p.Dir, ".zdev", "commands", name+".just")
	if _, err := os.Stat(justfilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not found, not an error
		}
		return nil, err
	}
	return &JustfileInfo{
		Name: name,
		Path: justfilePath,
	}, nil
}

// DiscoverJustfiles finds all .just files in .zdev/commands/
func (p *Project) DiscoverJustfiles() ([]JustfileInfo, error) {
	commandsDir := filepath.Join(p.Dir, ".zdev", "commands")

	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No commands directory is OK
		}
		return nil, err
	}

	var justfiles []JustfileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".just") {
			justfiles = append(justfiles, JustfileInfo{
				Name: strings.TrimSuffix(name, ".just"),
				Path: filepath.Join(commandsDir, name),
			})
		}
	}
	return justfiles, nil
}

// BuildJustEnv creates the environment variables to pass to just
func (p *Project) BuildJustEnv() map[string]string {
	env := make(map[string]string)

	// Copy current environment
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	// Add zdev-specific variables (override any existing)
	env["PROJECTNAME"] = p.Config.Name
	env["PROJECTPATH"] = p.Dir
	env["PROJECTDIR"] = filepath.Base(p.Dir)
	env["ZDEV_DOMAIN"] = config.GetZdevDomain()
	env["ZDEV_HOME"] = config.GetZdevHome()

	// Add project environment variables from config
	for k, v := range p.Config.Environment {
		env[k] = v
	}

	return env
}

// GetJustfileFromDir returns the justfile for a command name from a specific directory
// This is useful when we don't have a full project loaded
func GetJustfileFromDir(dir, name string) (*JustfileInfo, error) {
	justfilePath := filepath.Join(dir, ".zdev", "commands", name+".just")
	if _, err := os.Stat(justfilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not found, not an error
		}
		return nil, err
	}
	return &JustfileInfo{
		Name: name,
		Path: justfilePath,
	}, nil
}
