package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed templates/global-config.yaml
var defaultGlobalConfig string

//go:embed templates/traefik-dynamic.yaml
var traefikDynamicConfig string

//go:embed templates/docs.html
var docsHTMLTemplate string

// ProjectInfo holds project display information for the docs page
type ProjectInfo struct {
	Name    string
	Domain  string
	Path    string
	Running bool
}

// LoadProject loads and parses a project config from the given directory
func LoadProject(projectDir string) (*ProjectConfig, error) {
	configPath := filepath.Join(projectDir, ".scdev", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Build initial variables (PROJECTDIR, PROJECTPATH, etc. but not PROJECTNAME yet)
	vars := buildVariables(projectDir)
	dirName := vars["PROJECTDIR"]

	// First pass: substitute basic variables to resolve the name field
	content := substituteVariables(string(data), vars)

	// Parse to extract the name field
	var cfg ProjectConfig
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, formatConfigError(configPath, data, err)
	}

	// Determine the project name (use parsed name, or default to PROJECTDIR)
	// If name contains ${PROJECTNAME}, it's a circular reference - fall back to PROJECTDIR
	projectName := cfg.Name
	if projectName == "" || strings.Contains(projectName, "${PROJECTNAME}") {
		projectName = dirName
	}

	// Second pass: now substitute PROJECTNAME with the actual project name
	vars["PROJECTNAME"] = projectName
	content = substituteVariables(string(data), vars)

	// Re-parse with full variable substitution
	cfg = ProjectConfig{}
	decoder = yaml.NewDecoder(bytes.NewReader([]byte(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, formatConfigError(configPath, data, err)
	}

	// Set defaults
	if cfg.Name == "" {
		cfg.Name = projectName
	}

	// Default domain to {name}.{scdev_domain}
	if cfg.Domain == "" {
		cfg.Domain = cfg.Name + "." + DefaultDomain
	}

	// Default routing protocol to "http" when port is set
	for name, svc := range cfg.Services {
		if svc.Routing != nil && svc.Routing.Port != 0 && svc.Routing.Protocol == "" {
			svc.Routing.Protocol = "http"
			cfg.Services[name] = svc
		}
	}

	return &cfg, nil
}

// formatConfigError creates a user-friendly error message for config parsing failures
func formatConfigError(configPath string, data []byte, err error) error {
	errStr := err.Error()
	lines := strings.Split(string(data), "\n")

	// Check if it's an unknown field error from KnownFields(true)
	// yaml.v3 format: "yaml: unmarshal errors:\n  line X: field Y not found in type Z"
	if strings.Contains(errStr, "not found in type") {
		// Use regex to extract line number and field name
		re := regexp.MustCompile(`line (\d+): field (\S+) not found`)
		if matches := re.FindStringSubmatch(errStr); len(matches) == 3 {
			lineNum := 0
			fmt.Sscanf(matches[1], "%d", &lineNum)
			fieldName := matches[2]
			if lineNum > 0 && lineNum <= len(lines) {
				lineContent := strings.TrimSpace(lines[lineNum-1])
				return fmt.Errorf("%s:%d: unknown field %q\n  > %s", configPath, lineNum, fieldName, lineContent)
			}
		}
		return fmt.Errorf("%s: unknown field in config: %s", configPath, errStr)
	}

	// Check for YAML syntax errors with line numbers
	// yaml.v3 format: "yaml: line X: <description>"
	if strings.HasPrefix(errStr, "yaml: line ") {
		var lineNum int
		if _, scanErr := fmt.Sscanf(errStr, "yaml: line %d:", &lineNum); scanErr == nil && lineNum > 0 {
			// Extract the actual error message after "yaml: line X: "
			prefix := fmt.Sprintf("yaml: line %d: ", lineNum)
			message := strings.TrimPrefix(errStr, prefix)

			if lineNum <= len(lines) {
				lineContent := strings.TrimSpace(lines[lineNum-1])
				return fmt.Errorf("%s:%d: %s\n  > %s", configPath, lineNum, message, lineContent)
			}
			return fmt.Errorf("%s:%d: %s", configPath, lineNum, message)
		}
	}

	// Fallback: just add the config path
	return fmt.Errorf("%s: %s", configPath, errStr)
}

// buildVariables creates the variable map for substitution
// Note: PROJECTNAME is NOT set here - it's set after parsing the name field
func buildVariables(projectDir string) map[string]string {
	vars := make(map[string]string)

	// Built-in variables
	// PROJECTDIR = directory basename (always the directory name)
	// PROJECTNAME = will be set later to the parsed 'name:' field value
	vars["PROJECTDIR"] = filepath.Base(projectDir)
	vars["PROJECTPATH"] = projectDir
	vars["SCDEV_HOME"] = getScdevHome()
	vars["SCDEV_DOMAIN"] = getScdevDomain()
	vars["USER"] = os.Getenv("USER")
	vars["HOME"] = os.Getenv("HOME")

	// Include all environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			// Don't override built-in variables
			if _, exists := vars[parts[0]]; !exists {
				vars[parts[0]] = parts[1]
			}
		}
	}

	return vars
}

// substituteVariables replaces ${VAR} patterns with their values
func substituteVariables(content string, vars map[string]string) string {
	// Match ${VAR} pattern
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := match[2 : len(match)-1]

		if value, ok := vars[varName]; ok {
			return value
		}

		// Leave unresolved variables as-is (or could return empty string)
		return match
	})
}

// getScdevHome returns the scdev home directory
func getScdevHome() string {
	if home := os.Getenv("SCDEV_HOME"); home != "" {
		return home
	}
	return filepath.Join(os.Getenv("HOME"), ".scdev")
}

// GetScdevHome returns the scdev home directory (exported version)
func GetScdevHome() string {
	return getScdevHome()
}

// getScdevDomain returns the default domain
// Priority: 1. SCDEV_DOMAIN env var, 2. global config, 3. default
func getScdevDomain() string {
	// Check environment variable first
	if domain := os.Getenv("SCDEV_DOMAIN"); domain != "" {
		return domain
	}

	// Try to read from global config
	globalConfigPath := filepath.Join(getScdevHome(), GlobalConfigFilename)
	if data, err := os.ReadFile(globalConfigPath); err == nil {
		var globalCfg GlobalConfig
		if err := yaml.Unmarshal(data, &globalCfg); err == nil && globalCfg.Domain != "" {
			return globalCfg.Domain
		}
	}

	return DefaultDomain
}

// GetScdevDomain returns the default domain (exported version)
func GetScdevDomain() string {
	return getScdevDomain()
}

// LoadGlobalConfig loads the global config from ~/.scdev/global-config.yaml
// Returns default config if file doesn't exist
// defaultGlobalConfig returns a GlobalConfig with all defaults populated.
// Single source of truth — used for both "file missing" and "file exists" paths.
func newDefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		Version: 1,
		Domain:  DefaultDomain,
		Runtime: "docker",
		SSL: SSLConfig{
			Enabled: true,
		},
		Shared: SharedConfig{
			Router: RouterConfig{
				Image:     RouterImage,
				Dashboard: true,
			},
			Mail: MailConfig{
				Image: MailImage,
			},
			DBUI: DBUIConfig{
				Image: DBUIImage,
			},
			RedisInsights: RedisInsightsConfig{
				Image: RedisInsightsImage,
			},
			Observability: ObservabilityConfig{
				Image: ObservabilityImage,
			},
		},
		Terminal: TerminalConfig{
			Plain: false,
		},
	}
}

func LoadGlobalConfig() (*GlobalConfig, error) {
	globalConfigPath := filepath.Join(getScdevHome(), GlobalConfigFilename)

	data, err := os.ReadFile(globalConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := newDefaultGlobalConfig()
			return &cfg, nil
		}
		return nil, fmt.Errorf("failed to read global config: %w", err)
	}

	// Start with defaults, then let YAML override
	cfg := newDefaultGlobalConfig()

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse global config: %w", err)
	}

	return &cfg, nil
}

// EnsureGlobalConfig creates the global config file if it doesn't exist
// Returns the path to the config and whether it was newly created
func EnsureGlobalConfig() (string, bool, error) {
	scdevHome := getScdevHome()
	configPath := filepath.Join(scdevHome, GlobalConfigFilename)
	oldConfigPath := filepath.Join(scdevHome, "config.yaml")

	// Migrate old config.yaml to global-config.yaml if needed
	if _, err := os.Stat(oldConfigPath); err == nil {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// Old config exists but new doesn't - rename it
			if err := os.Rename(oldConfigPath, configPath); err != nil {
				return "", false, fmt.Errorf("failed to migrate config: %w", err)
			}
			return configPath, false, nil
		}
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return configPath, false, nil
	}

	// Create directory if needed
	if err := os.MkdirAll(scdevHome, 0755); err != nil {
		return "", false, fmt.Errorf("failed to create %s: %w", scdevHome, err)
	}

	// Generate config with current version constants
	configContent := generateDefaultGlobalConfig()

	// Write default config
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return "", false, fmt.Errorf("failed to write config: %w", err)
	}

	return configPath, true, nil
}

// generateDefaultGlobalConfig creates the default global config by substituting
// variables in the embedded template with values from defaults.go
func generateDefaultGlobalConfig() string {
	vars := map[string]string{
		"DefaultDomain":      DefaultDomain,
		"RouterImage":        RouterImage,
		"MailImage":          MailImage,
		"DBUIImage":          DBUIImage,
		"ObservabilityImage": ObservabilityImage,
	}
	return substituteConfigVars(defaultGlobalConfig, vars)
}

// substituteConfigVars replaces ${VAR} placeholders in config with values from vars map
func substituteConfigVars(content string, vars map[string]string) string {
	result := content
	for key, value := range vars {
		result = strings.ReplaceAll(result, "${"+key+"}", value)
	}
	return result
}

// projectDirOverride allows overriding the project directory discovery
// Set via SetProjectDirOverride, used by --config flag
var projectDirOverride string

// SetProjectDirOverride sets an explicit project directory, bypassing discovery
// The path should be the directory containing .scdev/ (not .scdev/ itself)
func SetProjectDirOverride(dir string) {
	projectDirOverride = dir
}

// GetProjectDirOverride returns the current project directory override, if set
func GetProjectDirOverride() string {
	return projectDirOverride
}

// FindProjectDir walks up from the current directory to find a .scdev/config.yaml
// If a project directory override is set (via --config flag), it returns that instead
func FindProjectDir() (string, error) {
	// Check for explicit override first
	if projectDirOverride != "" {
		// Verify the config exists at the override path
		configPath := filepath.Join(projectDirOverride, ".scdev", "config.yaml")
		if _, err := os.Stat(configPath); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("no .scdev/config.yaml found at %s", projectDirOverride)
			}
			return "", fmt.Errorf("failed to access config at %s: %w", projectDirOverride, err)
		}
		return projectDirOverride, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Resolve symlinks (macOS has /var -> /private/var)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Get home directory to exclude ~/.scdev (global config, not a project)
	homeDir, _ := os.UserHomeDir()

	for {
		configPath := filepath.Join(dir, ".scdev", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			// Skip if this is the global scdev home (~/.scdev), not a project
			if homeDir != "" && dir == homeDir {
				// This is ~/.scdev/config.yaml - the global config, not a project
				// Continue searching in parent (will eventually fail at root)
			} else {
				return dir, nil
			}
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			return "", fmt.Errorf("no .scdev/config.yaml found in current directory or any parent")
		}
		dir = parent
	}
}

// GetTraefikConfigDir returns the path to the Traefik config directory
func GetTraefikConfigDir() string {
	return filepath.Join(getScdevHome(), "traefik")
}

// GetCertsDir returns the path to the certificates directory
func GetCertsDir() string {
	return filepath.Join(getScdevHome(), "certs")
}

// EnsureTraefikConfig creates the Traefik dynamic config file if certs exist
// Returns the config directory path, or empty string if certs don't exist
func EnsureTraefikConfig() (string, error) {
	certsDir := GetCertsDir()
	certPath := filepath.Join(certsDir, "cert.pem")
	keyPath := filepath.Join(certsDir, "key.pem")

	// Check if both cert files exist
	if _, err := os.Stat(certPath); err != nil {
		return "", nil // No certs, return empty (not an error)
	}
	if _, err := os.Stat(keyPath); err != nil {
		return "", nil // No key, return empty (not an error)
	}

	// Create traefik config directory
	traefikDir := GetTraefikConfigDir()
	if err := os.MkdirAll(traefikDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create traefik config directory: %w", err)
	}

	// Write the dynamic config file
	configPath := filepath.Join(traefikDir, "dynamic.yaml")
	if err := os.WriteFile(configPath, []byte(traefikDynamicConfig), 0644); err != nil {
		return "", fmt.Errorf("failed to write traefik config: %w", err)
	}

	return traefikDir, nil
}

// GetDocsDir returns the path to the docs directory
func GetDocsDir() string {
	return filepath.Join(getScdevHome(), "docs")
}

// EnsureDocsConfig creates the docs HTML file and Traefik routing config
// Returns the docs directory path
func EnsureDocsConfig(domain string, tlsEnabled bool) (string, error) {
	return EnsureDocsConfigWithProjects(domain, tlsEnabled, nil)
}

// EnsureDocsConfigWithProjects creates the docs HTML file with project information
// Returns the docs directory path
func EnsureDocsConfigWithProjects(domain string, tlsEnabled bool, projects []ProjectInfo) (string, error) {
	docsDir := GetDocsDir()
	traefikDir := GetTraefikConfigDir()

	// Create directories
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create docs directory: %w", err)
	}
	if err := os.MkdirAll(traefikDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create traefik config directory: %w", err)
	}

	// Determine protocol
	protocol := "http"
	if tlsEnabled {
		protocol = "https"
	}

	// Generate projects section HTML
	projectsSection := generateProjectsSection(projects, protocol)

	// Generate docs HTML with variable substitution
	vars := map[string]string{
		"DOMAIN":           domain,
		"PROTOCOL":         protocol,
		"PROJECTS_SECTION": projectsSection,
	}
	docsHTML := substituteConfigVars(docsHTMLTemplate, vars)

	// Write docs HTML
	htmlPath := filepath.Join(docsDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte(docsHTML), 0644); err != nil {
		return "", fmt.Errorf("failed to write docs HTML: %w", err)
	}

	// Generate Traefik docs routing config
	docsConfig := generateDocsTraefikConfig(domain, tlsEnabled)
	configPath := filepath.Join(traefikDir, "docs.yaml")
	if err := os.WriteFile(configPath, []byte(docsConfig), 0644); err != nil {
		return "", fmt.Errorf("failed to write docs traefik config: %w", err)
	}

	return docsDir, nil
}

// generateProjectsSection creates the HTML for the projects section
func generateProjectsSection(projects []ProjectInfo, protocol string) string {
	if len(projects) == 0 {
		return `<div class="section">
            <div class="section-label">Projects</div>
            <p class="no-projects">No projects configured yet. Create a project with <code>.scdev/config.yaml</code> and run <code>scdev start</code>.</p>
        </div>`
	}

	var sb strings.Builder
	sb.WriteString(`<div class="section">
            <div class="section-label">Projects</div>
            <div class="projects-list">
`)

	for _, p := range projects {
		statusClass := "stopped"
		statusText := "stopped"
		if p.Running {
			statusClass = "running"
			statusText = "running"
		}

		url := fmt.Sprintf("%s://%s", protocol, p.Domain)

		sb.WriteString(fmt.Sprintf(`                <a href="%s" class="project-row %s">
                    <div class="project-name">
                        <h3>%s</h3>
                        <span class="project-status %s">%s</span>
                    </div>
                    <div class="project-meta">
                        <span class="url">%s</span>
                        <span class="path">%s</span>
                    </div>
                    <span class="project-open">open &rarr;</span>
                </a>
`, url, statusClass, escapeHTML(p.Name), statusClass, statusText, escapeHTML(p.Domain), escapeHTML(p.Path)))
	}

	sb.WriteString(`            </div>
        </div>`)

	return sb.String()
}

// escapeHTML escapes special HTML characters
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// UpdateDocsProjects updates only the docs HTML with current project information
// This is a lightweight update that doesn't recreate the Traefik config
func UpdateDocsProjects(domain string, tlsEnabled bool, projects []ProjectInfo) error {
	docsDir := GetDocsDir()

	// Ensure directory exists
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("failed to create docs directory: %w", err)
	}

	// Determine protocol
	protocol := "http"
	if tlsEnabled {
		protocol = "https"
	}

	// Generate projects section HTML
	projectsSection := generateProjectsSection(projects, protocol)

	// Generate docs HTML with variable substitution
	vars := map[string]string{
		"DOMAIN":           domain,
		"PROTOCOL":         protocol,
		"PROJECTS_SECTION": projectsSection,
	}
	docsHTML := substituteConfigVars(docsHTMLTemplate, vars)

	// Write docs HTML
	htmlPath := filepath.Join(docsDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte(docsHTML), 0644); err != nil {
		return fmt.Errorf("failed to write docs HTML: %w", err)
	}

	return nil
}

// generateDocsTraefikConfig creates the Traefik dynamic config for docs routing
func generateDocsTraefikConfig(domain string, tlsEnabled bool) string {
	protocol := "http"
	if tlsEnabled {
		protocol = "https"
	}

	// Escape dots in domain for regex
	escapedDomain := strings.ReplaceAll(domain, ".", "\\\\.")

	// Build config using fmt.Sprintf to handle backticks properly
	docsHost := "docs.shared." + domain
	redirectURL := protocol + "://docs.shared." + domain + "/"

	var sb strings.Builder
	sb.WriteString("# Auto-generated by scdev - do not edit manually\n")
	sb.WriteString("# Docs routing configuration\n\n")
	sb.WriteString("http:\n")
	sb.WriteString("  middlewares:\n")
	sb.WriteString("    # Statiq plugin serves static files from /docs\n")
	sb.WriteString("    scdev-docs:\n")
	sb.WriteString("      plugin:\n")
	sb.WriteString("        statiq:\n")
	sb.WriteString("          root: \"/docs\"\n\n")
	sb.WriteString("    # Redirect middleware for catch-all\n")
	sb.WriteString("    scdev-docs-redirect:\n")
	sb.WriteString("      redirectRegex:\n")
	sb.WriteString("        regex: \".*\"\n")
	sb.WriteString(fmt.Sprintf("        replacement: \"%s\"\n", redirectURL))
	sb.WriteString("        permanent: false\n\n")
	sb.WriteString("  routers:\n")
	sb.WriteString("    # Direct docs access (HTTP)\n")
	sb.WriteString("    scdev-docs-http:\n")
	sb.WriteString(fmt.Sprintf("      rule: \"Host(`%s`)\"\n", docsHost))
	sb.WriteString("      entryPoints:\n")
	sb.WriteString("        - http\n")
	sb.WriteString("      middlewares:\n")
	sb.WriteString("        - scdev-docs\n")
	sb.WriteString("      service: noop@internal\n")
	sb.WriteString("      priority: 100\n\n")
	sb.WriteString("    # Catch-all redirect (HTTP)\n")
	sb.WriteString("    scdev-catchall-http:\n")
	sb.WriteString(fmt.Sprintf("      rule: \"HostRegexp(`.*\\\\.%s`)\"\n", escapedDomain))
	sb.WriteString("      entryPoints:\n")
	sb.WriteString("        - http\n")
	sb.WriteString("      middlewares:\n")
	sb.WriteString("        - scdev-docs-redirect\n")
	sb.WriteString("      service: noop@internal\n")
	sb.WriteString("      priority: 1\n")

	// Add HTTPS routers if TLS is enabled
	if tlsEnabled {
		sb.WriteString("\n    # Direct docs access (HTTPS)\n")
		sb.WriteString("    scdev-docs-https:\n")
		sb.WriteString(fmt.Sprintf("      rule: \"Host(`%s`)\"\n", docsHost))
		sb.WriteString("      entryPoints:\n")
		sb.WriteString("        - https\n")
		sb.WriteString("      middlewares:\n")
		sb.WriteString("        - scdev-docs\n")
		sb.WriteString("      service: noop@internal\n")
		sb.WriteString("      priority: 100\n")
		sb.WriteString("      tls: {}\n\n")
		sb.WriteString("    # Catch-all redirect (HTTPS)\n")
		sb.WriteString("    scdev-catchall-https:\n")
		sb.WriteString(fmt.Sprintf("      rule: \"HostRegexp(`.*\\\\.%s`)\"\n", escapedDomain))
		sb.WriteString("      entryPoints:\n")
		sb.WriteString("        - https\n")
		sb.WriteString("      middlewares:\n")
		sb.WriteString("        - scdev-docs-redirect\n")
		sb.WriteString("      service: noop@internal\n")
		sb.WriteString("      priority: 1\n")
		sb.WriteString("      tls: {}\n")
	}

	return sb.String()
}
