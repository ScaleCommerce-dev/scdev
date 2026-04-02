//go:build integration

package project

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
	"github.com/ScaleCommerce-DEV/scdev/internal/state"
)

func TestProject_FullLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Load the minimal test fixture
	projectDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "minimal"))
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	proj, err := LoadFromDir(projectDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Verify project loaded correctly
	if proj.Config.Name != "minimal" {
		t.Errorf("expected project name 'minimal', got %q", proj.Config.Name)
	}

	if len(proj.Config.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(proj.Config.Services))
	}

	// Clean up any leftover containers from previous runs
	_ = proj.Down(ctx, false)

	// Test Start
	t.Log("Starting project...")
	if err := proj.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify container is running
	containerName := proj.ContainerName("app")
	running, err := proj.Runtime.IsContainerRunning(ctx, containerName)
	if err != nil {
		t.Fatalf("IsContainerRunning failed: %v", err)
	}
	if !running {
		t.Error("expected container to be running after Start")
	}

	// Test Stop
	t.Log("Stopping project...")
	if err := proj.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify container is stopped but exists
	running, err = proj.Runtime.IsContainerRunning(ctx, containerName)
	if err != nil {
		t.Fatalf("IsContainerRunning failed: %v", err)
	}
	if running {
		t.Error("expected container to not be running after Stop")
	}

	exists, err := proj.Runtime.ContainerExists(ctx, containerName)
	if err != nil {
		t.Fatalf("ContainerExists failed: %v", err)
	}
	if !exists {
		t.Error("expected container to still exist after Stop")
	}

	// Test Start again (should reuse container)
	t.Log("Restarting project...")
	if err := proj.Start(ctx); err != nil {
		t.Fatalf("Start (restart) failed: %v", err)
	}

	running, err = proj.Runtime.IsContainerRunning(ctx, containerName)
	if err != nil {
		t.Fatalf("IsContainerRunning failed: %v", err)
	}
	if !running {
		t.Error("expected container to be running after restart")
	}

	// Test Down
	t.Log("Removing project...")
	if err := proj.Down(ctx, false); err != nil {
		t.Fatalf("Down failed: %v", err)
	}

	// Verify container is removed
	exists, err = proj.Runtime.ContainerExists(ctx, containerName)
	if err != nil {
		t.Fatalf("ContainerExists failed: %v", err)
	}
	if exists {
		t.Error("expected container to not exist after Down")
	}

	// Verify project is unregistered from state
	stateMgr, err := state.DefaultManager()
	if err != nil {
		t.Fatalf("failed to get state manager: %v", err)
	}
	entry, err := stateMgr.GetProject(proj.Config.Name)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if entry != nil {
		t.Error("expected project to be unregistered from state after Down")
	}
}

func TestProject_MultiService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Load the full test fixture (has app + db)
	projectDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "full"))
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	proj, err := LoadFromDir(projectDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Verify project loaded correctly
	if proj.Config.Name != "full" {
		t.Errorf("expected project name 'full', got %q", proj.Config.Name)
	}

	if len(proj.Config.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(proj.Config.Services))
	}

	// Clean up any leftover containers
	_ = proj.Down(ctx, false)

	// Start project
	t.Log("Starting multi-service project...")
	if err := proj.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify both containers were created
	docker := runtime.NewDockerCLI()

	appContainer := proj.ContainerName("app")
	dbContainer := proj.ContainerName("db")

	appExists, err := docker.ContainerExists(ctx, appContainer)
	if err != nil {
		t.Fatalf("ContainerExists(app) failed: %v", err)
	}
	if !appExists {
		t.Error("expected app container to exist")
	}

	dbExists, err := docker.ContainerExists(ctx, dbContainer)
	if err != nil {
		t.Fatalf("ContainerExists(db) failed: %v", err)
	}
	if !dbExists {
		t.Error("expected db container to exist")
	}

	// db should be running (postgres stays up)
	dbRunning, err := docker.IsContainerRunning(ctx, dbContainer)
	if err != nil {
		t.Fatalf("IsContainerRunning(db) failed: %v", err)
	}
	if !dbRunning {
		t.Error("expected db container to be running")
	}

	// Clean up
	t.Log("Cleaning up...")
	if err := proj.Down(ctx, false); err != nil {
		t.Fatalf("Down failed: %v", err)
	}

	// Verify both removed
	appExists, _ = docker.ContainerExists(ctx, appContainer)
	dbExists, _ = docker.ContainerExists(ctx, dbContainer)

	if appExists || dbExists {
		t.Error("expected all containers to be removed after Down")
	}

	// Verify network removed
	networkExists, _ := docker.NetworkExists(ctx, proj.NetworkName())
	if networkExists {
		t.Error("expected network to be removed after Down")
	}
}

func TestProject_NetworkDNS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Load the full test fixture (has app + db)
	projectDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "full"))
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	proj, err := LoadFromDir(projectDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Clean up any leftover containers
	_ = proj.Down(ctx, false)

	// Start project
	t.Log("Starting project for DNS test...")
	if err := proj.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer proj.Down(ctx, false)

	// Verify network was created
	docker := runtime.NewDockerCLI()
	networkExists, err := docker.NetworkExists(ctx, proj.NetworkName())
	if err != nil {
		t.Fatalf("NetworkExists failed: %v", err)
	}
	if !networkExists {
		t.Fatal("expected network to exist")
	}

	// Test DNS resolution from db container (it stays running)
	// The db container should be able to resolve "db" (itself)
	t.Log("Testing DNS resolution...")
	err = docker.Exec(ctx, proj.ContainerName("db"), []string{"getent", "hosts", "db"}, false, runtime.ExecOptions{})
	if err != nil {
		t.Errorf("DNS resolution for 'db' failed: %v", err)
	}

	// Verify the alias is set by checking container inspect
	container, err := docker.GetContainer(ctx, proj.ContainerName("db"))
	if err != nil {
		t.Fatalf("GetContainer failed: %v", err)
	}
	if container == nil {
		t.Fatal("expected container to exist")
	}
	if !strings.Contains(container.Name, "db") {
		t.Errorf("expected container name to contain 'db', got %s", container.Name)
	}
}

func TestProject_Volumes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Load the full test fixture (has volumes defined)
	projectDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "projects", "full"))
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	proj, err := LoadFromDir(projectDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	docker := runtime.NewDockerCLI()

	dbDataVolume := proj.VolumeName("db_data")
	nodeModulesVolume := proj.VolumeName("node_modules")

	// Clean up any leftover resources for a clean test
	_ = proj.Down(ctx, true)

	// Verify volumes don't exist yet
	exists, _ := docker.VolumeExists(ctx, dbDataVolume)
	if exists {
		t.Error("expected db_data volume to not exist before start")
	}

	// Start project (should create volumes)
	t.Log("Starting project...")
	if err := proj.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify volumes were created
	exists, err = docker.VolumeExists(ctx, dbDataVolume)
	if err != nil {
		t.Fatalf("VolumeExists failed: %v", err)
	}
	if !exists {
		t.Error("expected db_data volume to exist after start")
	}

	exists, err = docker.VolumeExists(ctx, nodeModulesVolume)
	if err != nil {
		t.Fatalf("VolumeExists failed: %v", err)
	}
	if !exists {
		t.Error("expected node_modules volume to exist after start")
	}

	// Test Volumes() method
	volumes, err := proj.Volumes(ctx)
	if err != nil {
		t.Fatalf("Volumes() failed: %v", err)
	}
	if len(volumes) != 2 {
		t.Errorf("expected 2 volumes, got %d", len(volumes))
	}

	// Down without -v should keep volumes
	t.Log("Down without removing volumes...")
	if err := proj.Down(ctx, false); err != nil {
		t.Fatalf("Down failed: %v", err)
	}

	exists, _ = docker.VolumeExists(ctx, dbDataVolume)
	if !exists {
		t.Error("expected db_data volume to still exist after Down without -v")
	}

	exists, _ = docker.VolumeExists(ctx, nodeModulesVolume)
	if !exists {
		t.Error("expected node_modules volume to still exist after Down without -v")
	}

	// Down with -v should remove all volumes
	t.Log("Down with removing volumes...")
	if err := proj.Down(ctx, true); err != nil {
		t.Fatalf("Down with volumes failed: %v", err)
	}

	// Both volumes should be removed
	exists, _ = docker.VolumeExists(ctx, dbDataVolume)
	if exists {
		t.Error("expected db_data volume to be removed")
	}

	exists, _ = docker.VolumeExists(ctx, nodeModulesVolume)
	if exists {
		t.Error("expected node_modules volume to be removed")
	}
}
