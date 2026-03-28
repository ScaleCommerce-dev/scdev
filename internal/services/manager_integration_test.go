//go:build integration

package services

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
)

// Integration tests require Docker to be running
// Run with: go test -tags=integration ./...

func TestManager_RouterLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create manager with default config (uses DefaultDomain which resolves to 127.0.0.1)
	cfg := &config.GlobalConfig{
		Version: 1,
		Domain:  config.DefaultDomain,
		Runtime: "docker",
		Shared: config.SharedConfig{
			Router: config.RouterConfig{
				Image:     config.RouterImage,
				Dashboard: false,
			},
		},
	}

	mgr := NewManager(cfg)
	docker := runtime.NewDockerCLI()

	// Cleanup any leftover resources from previous runs
	_ = docker.StopContainer(ctx, RouterContainerName)
	_ = docker.RemoveContainer(ctx, RouterContainerName)
	_ = docker.RemoveNetwork(ctx, SharedNetworkName)

	// Test: Ensure shared network
	t.Run("EnsureSharedNetwork", func(t *testing.T) {
		if err := mgr.EnsureSharedNetwork(ctx); err != nil {
			t.Fatalf("EnsureSharedNetwork failed: %v", err)
		}

		exists, err := docker.NetworkExists(ctx, SharedNetworkName)
		if err != nil {
			t.Fatalf("NetworkExists failed: %v", err)
		}
		if !exists {
			t.Fatal("expected shared network to exist")
		}
	})

	// Test: Start router
	t.Run("StartRouter", func(t *testing.T) {
		if err := mgr.StartRouter(ctx); err != nil {
			t.Fatalf("StartRouter failed: %v", err)
		}

		status, err := mgr.RouterStatus(ctx)
		if err != nil {
			t.Fatalf("RouterStatus failed: %v", err)
		}
		if !status.Running {
			t.Fatal("expected router to be running")
		}
	})

	// Test: Start router again (should be idempotent)
	t.Run("StartRouter_Idempotent", func(t *testing.T) {
		if err := mgr.StartRouter(ctx); err != nil {
			t.Fatalf("StartRouter (second time) failed: %v", err)
		}
	})

	// Test: Connect to project network
	t.Run("ConnectToProject", func(t *testing.T) {
		// Create a test network
		testNetwork := "scdev_test_project"
		_ = docker.RemoveNetwork(ctx, testNetwork)
		if err := docker.CreateNetwork(ctx, testNetwork); err != nil {
			t.Fatalf("CreateNetwork failed: %v", err)
		}
		defer docker.RemoveNetwork(ctx, testNetwork)

		if err := mgr.ConnectToProject(ctx, testNetwork); err != nil {
			t.Fatalf("ConnectToProject failed: %v", err)
		}

		// Connect again should not error
		if err := mgr.ConnectToProject(ctx, testNetwork); err != nil {
			t.Fatalf("ConnectToProject (second time) failed: %v", err)
		}

		// Disconnect
		if err := mgr.DisconnectFromProject(ctx, testNetwork); err != nil {
			t.Fatalf("DisconnectFromProject failed: %v", err)
		}
	})

	// Test: Stop router
	t.Run("StopRouter", func(t *testing.T) {
		if err := mgr.StopRouter(ctx); err != nil {
			t.Fatalf("StopRouter failed: %v", err)
		}

		status, err := mgr.RouterStatus(ctx)
		if err != nil {
			t.Fatalf("RouterStatus failed: %v", err)
		}
		if status.Running {
			t.Fatal("expected router to be stopped")
		}
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		_ = docker.RemoveContainer(ctx, RouterContainerName)
		_ = docker.RemoveNetwork(ctx, SharedNetworkName)
	})
}

func TestManager_NetworkConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	docker := runtime.NewDockerCLI()
	testNetwork := "scdev_network_test"
	testContainer := "scdev_network_test_container"

	// Cleanup
	defer func() {
		_ = docker.StopContainer(ctx, testContainer)
		_ = docker.RemoveContainer(ctx, testContainer)
		_ = docker.RemoveNetwork(ctx, testNetwork)
	}()

	// Create network
	_ = docker.RemoveNetwork(ctx, testNetwork)
	if err := docker.CreateNetwork(ctx, testNetwork); err != nil {
		t.Fatalf("CreateNetwork failed: %v", err)
	}

	// Create and start a container on default network
	cfg := runtime.ContainerConfig{
		Name:    testContainer,
		Image:   config.TestImage,
		Command: []string{"sleep", "infinity"},
	}

	// Pull image if needed
	imageExists, _ := docker.ImageExists(ctx, cfg.Image)
	if !imageExists {
		if err := docker.PullImage(ctx, cfg.Image); err != nil {
			t.Fatalf("PullImage failed: %v", err)
		}
	}

	if _, err := docker.CreateContainer(ctx, cfg); err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	if err := docker.StartContainer(ctx, testContainer); err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	// Test: Connect container to network
	if err := docker.NetworkConnect(ctx, testNetwork, testContainer); err != nil {
		t.Fatalf("NetworkConnect failed: %v", err)
	}

	// Test: Disconnect container from network
	if err := docker.NetworkDisconnect(ctx, testNetwork, testContainer); err != nil {
		t.Fatalf("NetworkDisconnect failed: %v", err)
	}

	// Test: Disconnect again should not error
	if err := docker.NetworkDisconnect(ctx, testNetwork, testContainer); err != nil {
		t.Fatalf("NetworkDisconnect (second time) should not error: %v", err)
	}
}

func TestManager_DocsRoutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create manager with default config
	cfg := &config.GlobalConfig{
		Version: 1,
		Domain:  config.DefaultDomain,
		Runtime: "docker",
		Shared: config.SharedConfig{
			Router: config.RouterConfig{
				Image:     config.RouterImage,
				Dashboard: false,
			},
		},
	}

	mgr := NewManager(cfg)
	docker := runtime.NewDockerCLI()

	// Cleanup any leftover resources from previous runs
	_ = docker.StopContainer(ctx, RouterContainerName)
	_ = docker.RemoveContainer(ctx, RouterContainerName)
	_ = docker.RemoveNetwork(ctx, SharedNetworkName)

	// Ensure shared network exists
	if err := mgr.EnsureSharedNetwork(ctx); err != nil {
		t.Fatalf("EnsureSharedNetwork failed: %v", err)
	}

	// Start router (this will set up docs config via EnsureDocsConfig)
	if err := mgr.StartRouter(ctx); err != nil {
		t.Fatalf("StartRouter failed: %v", err)
	}

	// Use a client that doesn't follow redirects to check 302 status
	noRedirectClient := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	normalClient := &http.Client{Timeout: 5 * time.Second}

	// Test: docs.shared.<domain> returns 200
	t.Run("DocsReturns200", func(t *testing.T) {
		url := fmt.Sprintf("http://docs.shared.%s", cfg.Domain)

		var lastErr error
		for i := 0; i < 15; i++ {
			time.Sleep(1 * time.Second)

			resp, err := normalClient.Get(url)
			if err != nil {
				lastErr = fmt.Errorf("HTTP request to docs failed: %w", err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Logf("Docs route returns 200: %s", url)
				return
			}

			lastErr = fmt.Errorf("expected HTTP 200, got %d", resp.StatusCode)
		}

		t.Fatalf("Docs route check failed after retries: %v", lastErr)
	})

	// Test: nonexisting.<domain> returns 302 redirect to docs
	t.Run("NonExistingReturns302", func(t *testing.T) {
		url := fmt.Sprintf("http://nonexisting.%s", cfg.Domain)

		var lastErr error
		for i := 0; i < 15; i++ {
			time.Sleep(1 * time.Second)

			resp, err := noRedirectClient.Get(url)
			if err != nil {
				lastErr = fmt.Errorf("HTTP request to nonexisting domain failed: %w", err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusFound {
				// Verify redirect location points to docs
				location := resp.Header.Get("Location")
				expectedLocation := fmt.Sprintf("http://docs.shared.%s/", cfg.Domain)
				if location != expectedLocation {
					t.Fatalf("expected redirect to %s, got %s", expectedLocation, location)
				}
				t.Logf("Non-existing route returns 302 redirect to docs: %s -> %s", url, location)
				return
			}

			lastErr = fmt.Errorf("expected HTTP 302, got %d", resp.StatusCode)
		}

		t.Fatalf("Non-existing route check failed after retries: %v", lastErr)
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		_ = docker.StopContainer(ctx, RouterContainerName)
		_ = docker.RemoveContainer(ctx, RouterContainerName)
		_ = docker.RemoveNetwork(ctx, SharedNetworkName)
	})
}

func TestManager_DBUILifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create manager with default config (uses DefaultDomain which resolves to 127.0.0.1)
	cfg := &config.GlobalConfig{
		Version: 1,
		Domain:  config.DefaultDomain,
		Runtime: "docker",
		Shared: config.SharedConfig{
			Router: config.RouterConfig{
				Image:     config.RouterImage,
				Dashboard: false,
			},
			DBUI: config.DBUIConfig{
				Image: config.DBUIImage,
			},
		},
	}

	mgr := NewManager(cfg)
	docker := runtime.NewDockerCLI()

	// Cleanup any leftover resources from previous runs
	_ = docker.StopContainer(ctx, DBUIContainerName)
	_ = docker.RemoveContainer(ctx, DBUIContainerName)
	_ = docker.StopContainer(ctx, RouterContainerName)
	_ = docker.RemoveContainer(ctx, RouterContainerName)
	_ = docker.RemoveNetwork(ctx, SharedNetworkName)

	// Ensure shared network exists
	if err := mgr.EnsureSharedNetwork(ctx); err != nil {
		t.Fatalf("EnsureSharedNetwork failed: %v", err)
	}

	// Start router first (needed for HTTP routing to DBUI)
	if err := mgr.StartRouter(ctx); err != nil {
		t.Fatalf("StartRouter failed: %v", err)
	}

	// Test: Start DBUI
	t.Run("StartDBUI", func(t *testing.T) {
		if err := mgr.StartDBUI(ctx); err != nil {
			t.Fatalf("StartDBUI failed: %v", err)
		}

		status, err := mgr.DBUIStatus(ctx)
		if err != nil {
			t.Fatalf("DBUIStatus failed: %v", err)
		}
		if !status.Running {
			t.Fatal("expected DBUI to be running")
		}
	})

	// Test: Start DBUI again (should be idempotent)
	t.Run("StartDBUI_Idempotent", func(t *testing.T) {
		if err := mgr.StartDBUI(ctx); err != nil {
			t.Fatalf("StartDBUI (second time) failed: %v", err)
		}
	})

	// Test: HTTP health check - verifies Adminer loads without PHP errors
	t.Run("HTTPHealthCheck", func(t *testing.T) {
		// Test HTTP routing via Traefik with retries (Traefik needs time to pick up labels)
		// DefaultDomain (scalecommerce.site) is a wildcard that resolves to 127.0.0.1
		url := fmt.Sprintf("http://db.shared.%s", cfg.Domain)
		client := &http.Client{Timeout: 5 * time.Second}

		var lastErr error
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)

			resp, err := client.Get(url)
			if err != nil {
				lastErr = fmt.Errorf("HTTP request to Adminer failed: %w", err)
				continue
			}
			resp.Body.Close()

			// Check for 200 OK - a PHP fatal error would result in 500
			if resp.StatusCode == http.StatusOK {
				t.Logf("Adminer HTTP health check passed: %s", url)
				return
			}

			lastErr = fmt.Errorf("expected HTTP 200, got %d (possible PHP error in login-servers.php)", resp.StatusCode)
		}

		t.Fatalf("Adminer HTTP health check failed after retries: %v", lastErr)
	})

	// Test: Connect to project network
	t.Run("ConnectDBUIToProject", func(t *testing.T) {
		// Create a test network
		testNetwork := "scdev_dbui_test_project"
		_ = docker.RemoveNetwork(ctx, testNetwork)
		if err := docker.CreateNetwork(ctx, testNetwork); err != nil {
			t.Fatalf("CreateNetwork failed: %v", err)
		}
		defer docker.RemoveNetwork(ctx, testNetwork)

		if err := mgr.ConnectDBUIToProject(ctx, testNetwork); err != nil {
			t.Fatalf("ConnectDBUIToProject failed: %v", err)
		}

		// Connect again should not error
		if err := mgr.ConnectDBUIToProject(ctx, testNetwork); err != nil {
			t.Fatalf("ConnectDBUIToProject (second time) failed: %v", err)
		}

		// Disconnect
		if err := mgr.DisconnectDBUIFromProject(ctx, testNetwork); err != nil {
			t.Fatalf("DisconnectDBUIFromProject failed: %v", err)
		}
	})

	// Test: Stop DBUI
	t.Run("StopDBUI", func(t *testing.T) {
		if err := mgr.StopDBUI(ctx); err != nil {
			t.Fatalf("StopDBUI failed: %v", err)
		}

		status, err := mgr.DBUIStatus(ctx)
		if err != nil {
			t.Fatalf("DBUIStatus failed: %v", err)
		}
		if status.Running {
			t.Fatal("expected DBUI to be stopped")
		}
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		_ = docker.StopContainer(ctx, DBUIContainerName)
		_ = docker.RemoveContainer(ctx, DBUIContainerName)
		_ = docker.StopContainer(ctx, RouterContainerName)
		_ = docker.RemoveContainer(ctx, RouterContainerName)
		_ = docker.RemoveNetwork(ctx, SharedNetworkName)
	})
}
