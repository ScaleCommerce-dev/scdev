//go:build integration

package project

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	dockerRuntime "github.com/ScaleCommerce-DEV/scdev/internal/runtime"
)

// getMutagenBinaryPath returns the path to the mutagen binary
// It checks scdev's bin directory first, then PATH
func getMutagenBinaryPath() string {
	homeDir, err := os.UserHomeDir()
	if err == nil {
		scdevPath := filepath.Join(homeDir, ".scdev", "bin", "mutagen")
		if _, err := os.Stat(scdevPath); err == nil {
			return scdevPath
		}
	}

	// Check PATH
	path, err := exec.LookPath("mutagen")
	if err == nil {
		return path
	}

	return ""
}

// skipIfMutagenNotAvailable skips the test if mutagen is not installed
func skipIfMutagenNotAvailable(t *testing.T) string {
	// Mutagen is only auto-enabled on macOS
	if runtime.GOOS != "darwin" {
		t.Skip("Mutagen tests only run on macOS (where Mutagen is auto-enabled)")
	}

	mutagenPath := getMutagenBinaryPath()
	if mutagenPath == "" {
		t.Skip("mutagen binary not found, skipping Mutagen integration test")
	}

	return mutagenPath
}

func TestProject_MutagenLifecycle(t *testing.T) {
	mutagenPath := skipIfMutagenNotAvailable(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Load the mutagen test fixture
	projectDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "mutagen"))
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	proj, err := LoadFromDir(projectDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Verify project loaded correctly
	if proj.Config.Name != "mutagen-test" {
		t.Errorf("expected project name 'mutagen-test', got %q", proj.Config.Name)
	}

	// Verify Mutagen is enabled (should be on macOS)
	if !proj.IsMutagenEnabled() {
		t.Skip("Mutagen is not enabled for this project")
	}

	docker := dockerRuntime.NewDockerCLI()

	// Clean up any leftover resources from previous runs
	_ = proj.Down(ctx, true)

	// Expected resource names
	containerName := proj.ContainerName("app")
	sessionName := "scdev-mutagen-test-app" // Pattern: scdev-<project>-<service>

	// Start the project (should create container and Mutagen session)
	t.Log("Starting project with Mutagen...")
	if err := proj.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify container is running
	running, err := docker.IsContainerRunning(ctx, containerName)
	if err != nil {
		t.Fatalf("IsContainerRunning failed: %v", err)
	}
	if !running {
		t.Error("expected container to be running after Start")
	}

	// Verify Mutagen session was created
	checkSession := exec.CommandContext(ctx, mutagenPath, "sync", "list", sessionName)
	if output, err := checkSession.CombinedOutput(); err != nil {
		t.Logf("Mutagen session check output: %s", output)
		t.Errorf("expected Mutagen session '%s' to exist after Start: %v", sessionName, err)
	} else {
		t.Logf("Mutagen session '%s' exists", sessionName)
	}

	// Create a test file in project directory and verify sync
	testFile := filepath.Join(projectDir, "mutagen-test-file.txt")
	if err := os.WriteFile(testFile, []byte("hello from host"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Flush Mutagen to ensure sync completes
	t.Log("Flushing Mutagen session...")
	flushCmd := exec.CommandContext(ctx, mutagenPath, "sync", "flush", sessionName)
	if output, err := flushCmd.CombinedOutput(); err != nil {
		t.Logf("Flush output: %s", output)
		t.Errorf("failed to flush Mutagen session: %v", err)
	}

	// Verify file exists in container
	verifyCmd := exec.CommandContext(ctx, "docker", "exec", containerName, "cat", "/app/mutagen-test-file.txt")
	output, err := verifyCmd.CombinedOutput()
	if err != nil {
		t.Errorf("failed to verify file in container: %v (output: %s)", err, output)
	} else if string(output) != "hello from host" {
		t.Errorf("expected file content 'hello from host', got %q", string(output))
	}

	// Test Stop (should pause Mutagen session)
	t.Log("Stopping project...")
	if err := proj.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Container should be stopped
	running, _ = docker.IsContainerRunning(ctx, containerName)
	if running {
		t.Error("expected container to be stopped after Stop")
	}

	// Mutagen session should still exist (paused)
	checkSession = exec.CommandContext(ctx, mutagenPath, "sync", "list", sessionName)
	if _, err := checkSession.CombinedOutput(); err != nil {
		t.Errorf("expected Mutagen session '%s' to still exist after Stop", sessionName)
	}

	// Test Start again (should resume Mutagen session)
	t.Log("Starting project again...")
	if err := proj.Start(ctx); err != nil {
		t.Fatalf("Start (resume) failed: %v", err)
	}

	// Container should be running again
	running, _ = docker.IsContainerRunning(ctx, containerName)
	if !running {
		t.Error("expected container to be running after resume")
	}

	// Test Down (should terminate Mutagen session)
	t.Log("Bringing down project...")
	if err := proj.Down(ctx, false); err != nil {
		t.Fatalf("Down failed: %v", err)
	}

	// Container should be removed
	exists, _ := docker.ContainerExists(ctx, containerName)
	if exists {
		t.Error("expected container to be removed after Down")
	}

	// Mutagen session should be terminated
	checkSession = exec.CommandContext(ctx, mutagenPath, "sync", "list", sessionName)
	if err := checkSession.Run(); err == nil {
		t.Errorf("expected Mutagen session '%s' to be terminated after Down", sessionName)
	}

	t.Log("Mutagen lifecycle test completed successfully")
}

func TestProject_MutagenVolumeCleanup(t *testing.T) {
	_ = skipIfMutagenNotAvailable(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Load the mutagen test fixture
	projectDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "mutagen"))
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	proj, err := LoadFromDir(projectDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	if !proj.IsMutagenEnabled() {
		t.Skip("Mutagen is not enabled for this project")
	}

	docker := dockerRuntime.NewDockerCLI()

	// Clean up any leftover resources
	_ = proj.Down(ctx, true)

	// Expected sync volume name
	syncVolumeName := "sync.app.mutagen-test.scdev"

	// Start the project
	t.Log("Starting project...")
	if err := proj.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify sync volume was created
	exists, err := docker.VolumeExists(ctx, syncVolumeName)
	if err != nil {
		t.Fatalf("VolumeExists failed: %v", err)
	}
	if !exists {
		t.Errorf("expected sync volume '%s' to exist after Start", syncVolumeName)
	}

	// Down without -v should keep sync volume
	t.Log("Down without -v...")
	if err := proj.Down(ctx, false); err != nil {
		t.Fatalf("Down failed: %v", err)
	}

	exists, _ = docker.VolumeExists(ctx, syncVolumeName)
	if !exists {
		t.Error("expected sync volume to still exist after Down without -v")
	}

	// Start again to recreate container
	if err := proj.Start(ctx); err != nil {
		t.Fatalf("Start (second) failed: %v", err)
	}

	// Down with -v should remove sync volume
	t.Log("Down with -v...")
	if err := proj.Down(ctx, true); err != nil {
		t.Fatalf("Down with -v failed: %v", err)
	}

	exists, _ = docker.VolumeExists(ctx, syncVolumeName)
	if exists {
		t.Error("expected sync volume to be removed after Down with -v")
	}

	t.Log("Mutagen volume cleanup test completed successfully")
}

func TestProject_MutagenIgnores(t *testing.T) {
	_ = skipIfMutagenNotAvailable(t)

	// Load the mutagen test fixture
	projectDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "mutagen"))
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	proj, err := LoadFromDir(projectDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Verify project Mutagen config
	if len(proj.Config.Mutagen.Ignore) != 2 {
		t.Errorf("expected 2 Mutagen ignores in config, got %d", len(proj.Config.Mutagen.Ignore))
	}

	// Check expected ignores
	ignores := proj.Config.Mutagen.Ignore
	hasVarCache := false
	hasLogPattern := false
	for _, ignore := range ignores {
		if ignore == "var/cache" {
			hasVarCache = true
		}
		if ignore == "*.log" {
			hasLogPattern = true
		}
	}

	if !hasVarCache {
		t.Error("expected 'var/cache' in Mutagen ignores")
	}
	if !hasLogPattern {
		t.Error("expected '*.log' in Mutagen ignores")
	}
}
