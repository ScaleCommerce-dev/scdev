package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	canonical, err := canonicalBinaryPath()
	if err != nil {
		return fmt.Errorf("cannot determine install dir: %w", err)
	}
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	// Silently migrate legacy installs (plain file at /usr/local/bin/scdev)
	// to the symlink layout so subsequent updates don't need sudo.
	if err := migrateIfNeeded(execPath, canonical); err != nil {
		return err
	}

	currentVersion := strings.TrimPrefix(Version, "v")

	fmt.Printf("Current version: %s\n", Version)
	fmt.Printf("Checking for updates...\n")

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

	fmt.Printf("Downloading %s...\n", assetName)

	if err := os.MkdirAll(filepath.Dir(canonical), 0o755); err != nil {
		return fmt.Errorf("cannot create install dir: %w", err)
	}
	tmpPath := canonical + ".update"
	if err := downloadFile(downloadURL, tmpPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download failed: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod failed: %w", err)
	}

	// Atomic replace of the canonical binary. Dir is user-owned, no sudo.
	if err := os.Rename(tmpPath, canonical); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("install failed: %w", err)
	}

	fmt.Printf("Updated to %s\n", release.TagName)
	return nil
}

// migrateIfNeeded ensures execPath resolves to canonical. If not, it copies
// the current binary to canonical and replaces execPath with a symlink.
// Emits no output on the happy path; a sudo password prompt may appear when
// execPath lives in a root-owned dir.
func migrateIfNeeded(execPath, canonical string) error {
	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		realPath = execPath
	}
	// Also evaluate canonical so /tmp vs /private/tmp (macOS) doesn't
	// cause a spurious mismatch.
	realCanonical, err := filepath.EvalSymlinks(canonical)
	if err != nil {
		realCanonical = canonical
	}
	if realPath == realCanonical {
		return nil // already migrated
	}

	if err := os.MkdirAll(filepath.Dir(canonical), 0o755); err != nil {
		return err
	}
	if err := copyFile(realPath, canonical); err != nil {
		return err
	}
	if err := os.Chmod(canonical, 0o755); err != nil {
		return err
	}
	return migrateToSymlink(execPath, canonical)
}

// canonicalBinaryPath returns the user-owned location where the real scdev
// binary should live.
func canonicalBinaryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".scdev", "bin", "scdev"), nil
}

// migrateToSymlink replaces linkPath with a symlink to target. Tries without
// sudo first; falls back to `sudo ln -sfn` if the parent dir isn't writable.
func migrateToSymlink(linkPath, target string) error {
	if existing, err := os.Readlink(linkPath); err == nil && existing == target {
		return nil // already the right symlink
	}

	if err := atomicSymlink(linkPath, target); err == nil {
		return nil
	}

	cmd := exec.Command("sudo", "ln", "-sfn", target, linkPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// copyFile copies src to dst. dst is truncated if it exists.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// atomicSymlink creates or replaces linkPath as a symlink to target using
// create-tmp + rename so there's no window where linkPath is missing.
// Fails if the parent directory isn't writable by the current user.
func atomicSymlink(linkPath, target string) error {
	tmp := linkPath + ".symlink.tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, linkPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
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
