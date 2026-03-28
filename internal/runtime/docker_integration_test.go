//go:build integration

package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
)

// Integration tests require Docker to be running
// Run with: go test -tags=integration ./...

func TestDockerCLI_ContainerLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	docker := NewDockerCLI()
	containerName := "scdev_test_integration"
	imageName := config.TestImage

	// Cleanup any leftover container from previous runs
	_ = docker.StopContainer(ctx, containerName)
	_ = docker.RemoveContainer(ctx, containerName)

	// Test: Container should not exist initially
	exists, err := docker.ContainerExists(ctx, containerName)
	if err != nil {
		t.Fatalf("ContainerExists failed: %v", err)
	}
	if exists {
		t.Fatal("expected container to not exist")
	}

	// Test: Pull image if needed
	imageExists, err := docker.ImageExists(ctx, imageName)
	if err != nil {
		t.Fatalf("ImageExists failed: %v", err)
	}
	if !imageExists {
		if err := docker.PullImage(ctx, imageName); err != nil {
			t.Fatalf("PullImage failed: %v", err)
		}
	}

	// Test: Create container
	cfg := ContainerConfig{
		Name:    containerName,
		Image:   imageName,
		Command: []string{"sleep", "infinity"},
		Labels: map[string]string{
			"scdev.test": "true",
		},
	}

	id, err := docker.CreateContainer(ctx, cfg)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty container ID")
	}

	// Test: Container should exist now
	exists, err = docker.ContainerExists(ctx, containerName)
	if err != nil {
		t.Fatalf("ContainerExists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected container to exist after creation")
	}

	// Test: Container should not be running yet
	running, err := docker.IsContainerRunning(ctx, containerName)
	if err != nil {
		t.Fatalf("IsContainerRunning failed: %v", err)
	}
	if running {
		t.Fatal("expected container to not be running before start")
	}

	// Test: Start container
	if err := docker.StartContainer(ctx, containerName); err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	// Test: Container should be running now
	running, err = docker.IsContainerRunning(ctx, containerName)
	if err != nil {
		t.Fatalf("IsContainerRunning failed: %v", err)
	}
	if !running {
		t.Fatal("expected container to be running after start")
	}

	// Test: GetContainer returns correct info
	container, err := docker.GetContainer(ctx, containerName)
	if err != nil {
		t.Fatalf("GetContainer failed: %v", err)
	}
	if container == nil {
		t.Fatal("expected non-nil container")
	}
	if container.Name != containerName {
		t.Errorf("expected name %q, got %q", containerName, container.Name)
	}
	if !container.Running {
		t.Error("expected container.Running to be true")
	}

	// Test: Stop container
	if err := docker.StopContainer(ctx, containerName); err != nil {
		t.Fatalf("StopContainer failed: %v", err)
	}

	running, err = docker.IsContainerRunning(ctx, containerName)
	if err != nil {
		t.Fatalf("IsContainerRunning failed: %v", err)
	}
	if running {
		t.Fatal("expected container to not be running after stop")
	}

	// Test: Remove container
	if err := docker.RemoveContainer(ctx, containerName); err != nil {
		t.Fatalf("RemoveContainer failed: %v", err)
	}

	exists, err = docker.ContainerExists(ctx, containerName)
	if err != nil {
		t.Fatalf("ContainerExists failed: %v", err)
	}
	if exists {
		t.Fatal("expected container to not exist after removal")
	}
}
