package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/firstrun"
	"github.com/ScaleCommerce-DEV/scdev/internal/services"
	"github.com/ScaleCommerce-DEV/scdev/internal/ssl"
	"github.com/ScaleCommerce-DEV/scdev/internal/tools"
	"github.com/ScaleCommerce-DEV/scdev/internal/ui"
	"github.com/spf13/cobra"
)

// plainMode is cached for the systemcheck command
var sysPlainMode bool

var installCAFlag bool

var systemcheckCmd = &cobra.Command{
	Use:   "systemcheck",
	Short: "Check system dependencies and scdev setup",
	Long: `Verify that all required dependencies are installed and configured correctly.

This command checks:
- Docker availability
- mkcert installation
- Local CA installation
- SSL certificates
- Router status

On first run, this command will also perform initial setup including
downloading mkcert and generating SSL certificates.`,
	RunE: runSystemcheck,
}

func init() {
	systemcheckCmd.Flags().BoolVar(&installCAFlag, "install-ca", false, "Install the local CA certificate")
	rootCmd.AddCommand(systemcheckCmd)
}

// RunSystemcheckIfNeeded runs systemcheck if scdev is not initialized
// Returns true if systemcheck was run, false if already initialized
func RunSystemcheckIfNeeded() (bool, error) {
	// Ensure global config exists
	configPath, created, err := config.EnsureGlobalConfig()
	if err != nil {
		fmt.Printf("Warning: could not create global config: %v\n", err)
	} else if created {
		fmt.Printf("Created default global config: %s\n", configPath)
		fmt.Println("You can edit this file to customize scdev settings.")
		fmt.Println()
	}

	// Load global config
	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return false, fmt.Errorf("failed to load global config: %w", err)
	}

	// Check if first-run setup is needed
	firstrunMgr, err := firstrun.NewManager(globalCfg)
	if err != nil {
		return false, fmt.Errorf("failed to initialize first-run manager: %w", err)
	}

	if firstrunMgr.IsInitialized() {
		return false, nil
	}

	// Run first-time setup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	setupRan, err := firstrunMgr.RunSetup(ctx)
	if err != nil {
		return true, fmt.Errorf("first-run setup failed: %w", err)
	}

	// If setup completed successfully, start services and open docs
	if setupRan {
		openDocsAfterFirstRun(ctx, globalCfg)
	}

	return true, nil
}

// openDocsAfterFirstRun starts shared services and opens the docs page
func openDocsAfterFirstRun(ctx context.Context, cfg *config.GlobalConfig) {
	fmt.Println("Starting shared services...")
	fmt.Println()

	mgr := services.NewManager(cfg)

	// Start router (required for docs)
	if err := mgr.StartRouter(ctx); err != nil {
		fmt.Printf("Warning: could not start router: %v\n", err)
		return
	}

	// Start other services
	_ = mgr.StartMail(ctx)
	_ = mgr.StartDBUI(ctx)

	// Build docs URL
	protocol := "http"
	if cfg.SSL.Enabled {
		protocol = "https"
	}
	url := fmt.Sprintf("%s://docs.shared.%s", protocol, cfg.Domain)

	fmt.Println()
	fmt.Println("Opening documentation...")
	fmt.Println()

	// Open docs in browser
	if err := openBrowser(url); err != nil {
		fmt.Printf("Could not open browser. Visit: %s\n", url)
	}
}

func runSystemcheck(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Ensure global config exists
	configPath, created, err := config.EnsureGlobalConfig()
	if err != nil {
		fmt.Printf("Warning: could not create global config: %v\n", err)
	} else if created {
		fmt.Printf("Created default global config: %s\n", configPath)
		fmt.Println("You can edit this file to customize scdev settings.")
		fmt.Println()
	}

	// Load global config
	globalCfg, err := config.LoadGlobalConfig()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	// Set plain mode from config
	sysPlainMode = ui.PlainMode(globalCfg.Terminal.Plain)

	// Check if first-run setup is needed
	firstrunMgr, err := firstrun.NewManager(globalCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize first-run manager: %w", err)
	}

	if !firstrunMgr.IsInitialized() {
		// Run first-time setup
		setupRan, err := firstrunMgr.RunSetup(ctx)
		if err != nil {
			// First-run errors are not fatal - continue with systemcheck
			fmt.Printf("Warning: first-run setup incomplete: %v\n", err)
			fmt.Println()
		} else if setupRan {
			// Setup completed - start services and open docs
			openDocsAfterFirstRun(ctx, globalCfg)
		}
	}

	fmt.Println()
	fmt.Println("scdev System Check")
	fmt.Println("==================")
	fmt.Println()

	issues := 0

	// Check Docker
	issues += checkDocker(ctx)

	// Check mkcert
	mkcertPath, mkcertIssues := checkMkcert(ctx, globalCfg)
	issues += mkcertIssues

	// Handle --install-ca flag
	if installCAFlag && mkcertPath != "" {
		fmt.Println()
		fmt.Println("Installing local CA...")
		mkcert := tools.NewMkcert(mkcertPath)
		if err := mkcert.InstallCA(ctx); err != nil {
			fmt.Printf("  %s CA installation failed: %v\n", statusText("FAILED"), err)
			issues++
		} else {
			fmt.Printf("  %s CA installed successfully\n", statusText("OK"))
		}
		fmt.Println()
	}

	// Check local CA (only if mkcert is available)
	if mkcertPath != "" {
		issues += checkCA(ctx, mkcertPath, globalCfg)
	}

	// Check certificates
	issues += checkCertificates(globalCfg)

	// Check router status
	issues += checkRouter(ctx, globalCfg)

	// Summary
	fmt.Println()
	if issues == 0 {
		fmt.Printf("%s\n", ui.Color("All checks passed!", "green", sysPlainMode))
	} else {
		fmt.Printf("%s: %d\n", ui.Color("Issues found", "yellow", sysPlainMode), issues)
		if mkcertPath == "" {
			fmt.Println("Run 'scdev systemcheck' to download mkcert and complete setup.")
		} else if !installCAFlag {
			fmt.Println("Run 'scdev systemcheck --install-ca' to install the local CA.")
		}
	}
	fmt.Println()

	return nil
}

// statusText returns colored status text
func statusText(status string) string {
	var color string
	switch status {
	case "OK", "running":
		color = "green"
	case "MISSING", "FAILED", "ERROR", "stopped":
		color = "red"
	case "SKIP":
		color = "yellow"
	default:
		return status
	}
	return ui.Color(status, color, sysPlainMode)
}

func checkDocker(ctx context.Context) int {
	fmt.Print("Docker:        ")

	_, found := tools.FindInPath("docker")
	if !found {
		fmt.Printf("%s (not found in PATH)\n", statusText("MISSING"))
		return 1
	}

	version, err := tools.RunTool(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		fmt.Printf("%s (not running)\n", statusText("ERROR"))
		return 1
	}

	fmt.Printf("%s (version %s)\n", statusText("OK"), version)
	return 0
}

func checkMkcert(ctx context.Context, cfg *config.GlobalConfig) (string, int) {
	fmt.Print("mkcert:        ")

	if !cfg.SSL.Enabled {
		fmt.Printf("%s (SSL disabled in config)\n", statusText("SKIP"))
		return "", 0
	}

	// Check system PATH first
	if path, found := tools.FindInPath("mkcert"); found {
		version, err := tools.RunTool(ctx, path, "-version")
		if err != nil {
			version = "unknown"
		}
		fmt.Printf("%s (%s %s)\n", statusText("OK"), path, version)
		return path, 0
	}

	// Check scdev bin directory
	homeDir, _ := os.UserHomeDir()
	scdevBinPath := filepath.Join(homeDir, ".scdev", "bin", "mkcert")
	if _, err := os.Stat(scdevBinPath); err == nil {
		version, err := tools.RunTool(ctx, scdevBinPath, "-version")
		if err != nil {
			version = "unknown"
		}
		fmt.Printf("%s (%s %s)\n", statusText("OK"), scdevBinPath, version)
		return scdevBinPath, 0
	}

	fmt.Printf("%s (not installed)\n", statusText("MISSING"))
	fmt.Println("               Run 'scdev systemcheck' to download mkcert")
	return "", 1
}

func checkCA(ctx context.Context, mkcertPath string, cfg *config.GlobalConfig) int {
	fmt.Print("Local CA:      ")

	mkcert := tools.NewMkcert(mkcertPath)

	caRoot, err := mkcert.GetCARoot(ctx)
	if err != nil {
		fmt.Printf("%s (failed to get CA root: %v)\n", statusText("ERROR"), err)
		return 1
	}

	initialized, err := mkcert.IsCAInitialized(ctx)
	if err != nil {
		fmt.Printf("%s (failed to check: %v)\n", statusText("ERROR"), err)
		return 1
	}

	if !initialized {
		fmt.Printf("%s (not initialized)\n", statusText("MISSING"))
		fmt.Println("               Run 'scdev systemcheck --install-ca' to install")
		return 1
	}

	// CA files exist - also check if trusted by the system
	certPath := filepath.Join(config.GetCertsDir(), ssl.CertFileName)
	if _, err := os.Stat(certPath); err == nil {
		trusted, err := mkcert.IsCATrusted(ctx, certPath)
		if err == nil && !trusted {
			fmt.Printf("%s (%s - not trusted by system)\n", statusText("MISSING"), caRoot)
			fmt.Println("               Run 'scdev systemcheck --install-ca' to install")
			return 1
		}
	}

	fmt.Printf("%s (%s)\n", statusText("OK"), caRoot)
	return 0
}

func checkCertificates(cfg *config.GlobalConfig) int {
	fmt.Print("Certificates:  ")

	if !cfg.SSL.Enabled {
		fmt.Printf("%s (SSL disabled in config)\n", statusText("SKIP"))
		return 0
	}

	certsDir := config.GetCertsDir()
	certPath := filepath.Join(certsDir, "cert.pem")
	keyPath := filepath.Join(certsDir, "key.pem")

	certExists := false
	keyExists := false

	if _, err := os.Stat(certPath); err == nil {
		certExists = true
	}
	if _, err := os.Stat(keyPath); err == nil {
		keyExists = true
	}

	if !certExists || !keyExists {
		fmt.Printf("%s (not generated)\n", statusText("MISSING"))
		fmt.Println("               Run 'scdev systemcheck' to generate certificates")
		return 1
	}

	fmt.Printf("%s (*.%s)\n", statusText("OK"), cfg.Domain)
	return 0
}

func checkRouter(ctx context.Context, cfg *config.GlobalConfig) int {
	fmt.Print("Router:        ")

	mgr := services.NewManager(cfg)
	status, err := mgr.RouterStatus(ctx)
	if err != nil {
		fmt.Printf("%s (failed to check: %v)\n", statusText("ERROR"), err)
		return 1
	}

	if !status.Running {
		fmt.Printf("%s\n", statusText("stopped"))
		fmt.Println("               Run 'scdev services start' to start the router")
		return 0 // Not running is not an error, just informational
	}

	// Check if TLS is enabled
	tlsInfo := ""
	if cfg.SSL.Enabled {
		// Check if certs exist
		certsDir := config.GetCertsDir()
		certPath := filepath.Join(certsDir, ssl.CertFileName)
		if _, err := os.Stat(certPath); err == nil {
			tlsInfo = ", TLS"
		}
	}

	fmt.Printf("%s (ports 80, 443%s)\n", statusText("running"), tlsInfo)
	return 0
}
