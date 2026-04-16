package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/ScaleCommerce-DEV/scdev/internal/updatecheck"
	"github.com/spf13/cobra"
)

const selfUpdateGithubRepo = "ScaleCommerce-DEV/scdev"

// githubRelease is a minimal representation of a GitHub release.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update scdev to the latest version",
	Long:  "Checks GitHub for the latest release, downloads the matching binary, and replaces the current executable.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSelfUpdate()
	},
}

func init() {
	rootCmd.AddCommand(selfUpdateCmd)
}

func runSelfUpdate() error {
	currentVersion := strings.TrimPrefix(Version, "v")

	fmt.Printf("Current version: %s\n", Version)
	fmt.Printf("Checking for updates...\n")

	// Fetch latest release from GitHub
	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == currentVersion {
		fmt.Printf("Already up to date (%s)\n", Version)
		return nil
	}

	cmp, err := updatecheck.CompareSemver(currentVersion, latestVersion)
	if err == nil && cmp >= 0 {
		fmt.Printf("Already up to date (%s, latest: %s)\n", Version, release.TagName)
		return nil
	}

	fmt.Printf("New version available: %s\n", release.TagName)

	// Find the right asset for this OS/arch
	assetName := selfUpdateBinaryName()
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s (looked for %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}

	// Get path to current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	// Resolve symlinks so we replace the actual binary, not the symlink
	execPath, err = resolveExecPath(execPath)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	fmt.Printf("Downloading %s...\n", assetName)

	// Download to a temp file next to the current binary
	tmpPath := execPath + ".update"
	if err := downloadFile(downloadURL, tmpPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download failed: %w", err)
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod failed: %w", err)
	}

	// Replace current binary: rename old to .old, rename new to current, remove old
	oldPath := execPath + ".old"
	os.Remove(oldPath) // clean up any previous .old file

	if err := os.Rename(execPath, oldPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot replace binary (try running with sudo): %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		// Try to restore the old binary
		os.Rename(oldPath, execPath)
		return fmt.Errorf("cannot install new binary: %w", err)
	}

	os.Remove(oldPath)

	fmt.Printf("Updated to %s\n", release.TagName)
	return nil
}

func fetchLatestRelease() (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", selfUpdateGithubRepo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &release, nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func selfUpdateBinaryName() string {
	return "scdev-" + runtime.GOOS + "-" + runtime.GOARCH
}

// resolveExecPath follows symlinks to find the real binary path.
func resolveExecPath(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return path, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := os.Readlink(path)
		if err != nil {
			return path, err
		}
		return resolved, nil
	}
	return path, nil
}

