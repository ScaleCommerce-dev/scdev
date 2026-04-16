package project

import (
	"context"
	"testing"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
)

// =============================================================================
// computeConfigHash tests
// =============================================================================

func baseCfg() runtime.ContainerConfig {
	return runtime.ContainerConfig{
		Name:        "app.testproject.scdev",
		Image:       "alpine:3.18",
		WorkingDir:  "/app",
		Command:     []string{"sh", "-c", "sleep infinity"},
		NetworkName: "testproject.scdev",
		Aliases:     []string{"app"},
		Env: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
		Labels: map[string]string{
			"scdev.project": "testproject",
			"scdev.service": "app",
		},
		Volumes: []runtime.VolumeMount{
			{Source: "data.testproject.scdev", Target: "/data"},
		},
		Ports: []string{"127.0.0.1:5432:5432"},
	}
}

func TestComputeConfigHash_Deterministic(t *testing.T) {
	cfg := baseCfg()
	h1 := computeConfigHash(cfg)

	// Rebuild with maps inserted in different order - hash must match
	cfg2 := baseCfg()
	cfg2.Env = map[string]string{"BAZ": "qux", "FOO": "bar"}
	cfg2.Labels = map[string]string{"scdev.service": "app", "scdev.project": "testproject"}
	h2 := computeConfigHash(cfg2)

	if h1 != h2 {
		t.Errorf("hash not deterministic across map orderings: %s vs %s", h1, h2)
	}
}

func TestComputeConfigHash_ExcludesOwnLabel(t *testing.T) {
	cfg := baseCfg()
	h1 := computeConfigHash(cfg)

	cfg.Labels[configHashLabel] = "some-other-value"
	h2 := computeConfigHash(cfg)

	if h1 != h2 {
		t.Errorf("hash must ignore configHashLabel value: %s vs %s", h1, h2)
	}
}

func TestComputeConfigHash_DetectsChanges(t *testing.T) {
	base := computeConfigHash(baseCfg())

	cases := []struct {
		name   string
		mutate func(*runtime.ContainerConfig)
	}{
		{"image change", func(c *runtime.ContainerConfig) { c.Image = "alpine:3.19" }},
		{"env value change", func(c *runtime.ContainerConfig) { c.Env["FOO"] = "different" }},
		{"env key added", func(c *runtime.ContainerConfig) { c.Env["NEW"] = "1" }},
		{"env key removed", func(c *runtime.ContainerConfig) { delete(c.Env, "FOO") }},
		{"working dir change", func(c *runtime.ContainerConfig) { c.WorkingDir = "/elsewhere" }},
		{"command change", func(c *runtime.ContainerConfig) { c.Command = []string{"sh", "-c", "sleep 1"} }},
		{"volume source change", func(c *runtime.ContainerConfig) { c.Volumes[0].Source = "other.testproject.scdev" }},
		{"volume target change", func(c *runtime.ContainerConfig) { c.Volumes[0].Target = "/other" }},
		{"volume added", func(c *runtime.ContainerConfig) {
			c.Volumes = append(c.Volumes, runtime.VolumeMount{Source: "b", Target: "/b"})
		}},
		{"label added", func(c *runtime.ContainerConfig) { c.Labels["traefik.enable"] = "true" }},
		{"label removed", func(c *runtime.ContainerConfig) { delete(c.Labels, "scdev.service") }},
		{"port change", func(c *runtime.ContainerConfig) { c.Ports = []string{"127.0.0.1:5433:5432"} }},
		{"network change", func(c *runtime.ContainerConfig) { c.NetworkName = "other.scdev" }},
		{"alias change", func(c *runtime.ContainerConfig) { c.Aliases = []string{"renamed"} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseCfg()
			tc.mutate(&cfg)
			got := computeConfigHash(cfg)
			if got == base {
				t.Errorf("expected hash to change for %q, got same hash %s", tc.name, got)
			}
		})
	}
}

// =============================================================================
// serviceNeedsRecreate tests
// =============================================================================

// seedRunningService creates a project whose "app" container is already
// "running" in the mock with the labels we would have stamped at creation.
// svcOverride lets a test customise the service config before seeding so the
// seeded labels (including the hash) reflect that config.
func seedRunningService(t *testing.T, svcOverride func(*config.ServiceConfig)) (*Project, *runtime.MockRuntime) {
	t.Helper()
	cleanup := setupTestEnv(t)
	t.Cleanup(cleanup)

	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	svc := p.Config.Services["app"]
	if svcOverride != nil {
		svcOverride(&svc)
		p.Config.Services["app"] = svc
	}

	cfg := p.buildContainerConfig("app", svc, false, nil)
	mock.Containers[cfg.Name] = cfg
	mock.ContainersExist[cfg.Name] = true
	mock.ContainersRunning[cfg.Name] = true

	return p, mock
}

func TestServiceNeedsRecreate_NoDrift(t *testing.T) {
	p, _ := seedRunningService(t, nil)

	got, err := p.serviceNeedsRecreate(context.Background(), "app", p.Config.Services["app"])
	if err != nil {
		t.Fatalf("serviceNeedsRecreate error: %v", err)
	}
	if got {
		t.Error("expected no recreate when config is unchanged")
	}
}

func TestServiceNeedsRecreate_EnvChange(t *testing.T) {
	p, _ := seedRunningService(t, nil)

	// Change env on the service - should trigger recreation
	svc := p.Config.Services["app"]
	svc.Environment = map[string]string{"NEW_VAR": "value"}
	p.Config.Services["app"] = svc

	got, err := p.serviceNeedsRecreate(context.Background(), "app", svc)
	if err != nil {
		t.Fatalf("serviceNeedsRecreate error: %v", err)
	}
	if !got {
		t.Error("expected recreate after adding an env var")
	}
}

func TestServiceNeedsRecreate_ImageChange(t *testing.T) {
	p, _ := seedRunningService(t, nil)

	svc := p.Config.Services["app"]
	svc.Image = "alpine:3.20"
	p.Config.Services["app"] = svc

	got, err := p.serviceNeedsRecreate(context.Background(), "app", svc)
	if err != nil {
		t.Fatalf("serviceNeedsRecreate error: %v", err)
	}
	if !got {
		t.Error("expected recreate after image change")
	}
}

func TestServiceNeedsRecreate_VolumeChange(t *testing.T) {
	p, _ := seedRunningService(t, nil)

	svc := p.Config.Services["app"]
	svc.Volumes = []string{"data:/data", "newvol:/new"}
	p.Config.Services["app"] = svc

	got, err := p.serviceNeedsRecreate(context.Background(), "app", svc)
	if err != nil {
		t.Fatalf("serviceNeedsRecreate error: %v", err)
	}
	if !got {
		t.Error("expected recreate after adding a volume mount")
	}
}

func TestServiceNeedsRecreate_CommandChange(t *testing.T) {
	p, _ := seedRunningService(t, nil)

	svc := p.Config.Services["app"]
	svc.Command = "sleep 5"
	p.Config.Services["app"] = svc

	got, err := p.serviceNeedsRecreate(context.Background(), "app", svc)
	if err != nil {
		t.Fatalf("serviceNeedsRecreate error: %v", err)
	}
	if !got {
		t.Error("expected recreate after command change")
	}
}

func TestServiceNeedsRecreate_MissingHashLabelRecreates(t *testing.T) {
	// Simulate a container created before the hash label existed:
	// labels are present but scdev.config-hash is missing.
	p, mock := seedRunningService(t, nil)
	containerName := p.ContainerName("app")
	labels := mock.Containers[containerName].Labels
	delete(labels, configHashLabel)
	mock.ContainerLabels[containerName] = labels

	got, err := p.serviceNeedsRecreate(context.Background(), "app", p.Config.Services["app"])
	if err != nil {
		t.Fatalf("serviceNeedsRecreate error: %v", err)
	}
	if !got {
		t.Error("expected recreate when stamped hash label is missing (pre-upgrade container)")
	}
}

// =============================================================================
// Update end-to-end tests (via MockRuntime)
// =============================================================================

func TestUpdate_NoChangesReportsUpToDate(t *testing.T) {
	p, mock := seedRunningService(t, nil)
	// Mark network/volume as existing so Update takes the incremental path
	mock.NetworksExist[p.NetworkName()] = true
	mock.VolumesExist[p.VolumeName("data")] = true

	updated, err := p.Update(context.Background())
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated {
		t.Error("expected Update to report no changes when config matches running state")
	}
	if mock.CallCount("RemoveContainer") != 0 {
		t.Errorf("RemoveContainer called %d times, want 0", mock.CallCount("RemoveContainer"))
	}
}

func TestUpdate_RecreatesOnEnvChange(t *testing.T) {
	p, mock := seedRunningService(t, nil)
	mock.NetworksExist[p.NetworkName()] = true
	mock.VolumesExist[p.VolumeName("data")] = true

	// Change env - simulates user editing config and running `scdev update`
	svc := p.Config.Services["app"]
	svc.Environment = map[string]string{"SYMFONY_TRUSTED_PROXIES": "0.0.0.0/0"}
	p.Config.Services["app"] = svc

	updated, err := p.Update(context.Background())
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if !updated {
		t.Error("expected Update to report changes after env var added")
	}
	if mock.CallCount("RemoveContainer") != 1 {
		t.Errorf("RemoveContainer called %d times, want 1", mock.CallCount("RemoveContainer"))
	}
	if mock.CallCount("CreateContainer") != 1 {
		t.Errorf("CreateContainer called %d times, want 1", mock.CallCount("CreateContainer"))
	}
}
