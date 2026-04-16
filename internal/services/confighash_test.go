package services

import (
	"context"
	"testing"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
)

// TestSharedServiceConfigs_StampConfigHash verifies every shared-service
// *ContainerConfig function produces a container with a non-empty
// ConfigHashLabel. Without this label, startService can't detect drift.
func TestSharedServiceConfigs_StampConfigHash(t *testing.T) {
	cases := map[string]runtime.ContainerConfig{
		"mail":          MailContainerConfig(MailServiceConfig{Image: "axllent/mailpit:latest", Domain: "scalecommerce.site"}),
		"db":            DBUIContainerConfig(DBUIServiceConfig{Image: "adminer:latest", Domain: "scalecommerce.site"}),
		"redisInsights": RedisInsightsContainerConfig(RedisInsightsServiceConfig{Image: "redis/redisinsight:latest", Domain: "scalecommerce.site"}),
		"router":        RouterContainerConfig(RouterConfig{Image: config.RouterImage, Domain: "scalecommerce.site"}),
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if cfg.Labels[runtime.ConfigHashLabel] == "" {
				t.Errorf("%s container config missing %s label", name, runtime.ConfigHashLabel)
			}
		})
	}
}

// TestMailContainerConfig_HashChangesOnTLSFlip guards the user-visible
// bug that motivated this whole change: flipping TLS must bump the hash
// so startService notices the baked container is stale.
func TestMailContainerConfig_HashChangesOnTLSFlip(t *testing.T) {
	noTLS := MailContainerConfig(MailServiceConfig{Image: "axllent/mailpit:latest", Domain: "scalecommerce.site", TLSEnabled: false})
	withTLS := MailContainerConfig(MailServiceConfig{Image: "axllent/mailpit:latest", Domain: "scalecommerce.site", TLSEnabled: true})

	if noTLS.Labels[runtime.ConfigHashLabel] == withTLS.Labels[runtime.ConfigHashLabel] {
		t.Error("expected hash to differ when TLS flips on")
	}
}

// TestDBUIContainerConfig_HashChangesOnTLSFlip is the concrete
// regression test for the "HTTPS to db.shared redirects to docs"
// incident: the db container was created with TLSEnabled=false, hash
// stays matched to that broken state, but once SSL.Enabled flipped on
// the expected hash diverged - startService now recreates.
func TestDBUIContainerConfig_HashChangesOnTLSFlip(t *testing.T) {
	noTLS := DBUIContainerConfig(DBUIServiceConfig{Image: "adminer:latest", Domain: "scalecommerce.site", TLSEnabled: false})
	withTLS := DBUIContainerConfig(DBUIServiceConfig{Image: "adminer:latest", Domain: "scalecommerce.site", TLSEnabled: true})

	if noTLS.Labels[runtime.ConfigHashLabel] == withTLS.Labels[runtime.ConfigHashLabel] {
		t.Error("expected hash to differ when TLS flips on")
	}
}

// TestMailContainerConfig_HashChangesOnImageBump guards image drift.
func TestMailContainerConfig_HashChangesOnImageBump(t *testing.T) {
	v1 := MailContainerConfig(MailServiceConfig{Image: "axllent/mailpit:v1.0", Domain: "scalecommerce.site"})
	v2 := MailContainerConfig(MailServiceConfig{Image: "axllent/mailpit:v2.0", Domain: "scalecommerce.site"})

	if v1.Labels[runtime.ConfigHashLabel] == v2.Labels[runtime.ConfigHashLabel] {
		t.Error("expected hash to differ when image changes")
	}
}

// TestStartService_NoDriftNoRecreate: container exists, running, hash
// matches - startService is a pure no-op (no Remove, no Create).
func TestStartService_NoDriftNoRecreate(t *testing.T) {
	mock := runtime.NewMockRuntime()
	m := &Manager{
		cfg:     &config.GlobalConfig{Shared: config.SharedConfig{Mail: config.MailConfig{Image: "axllent/mailpit:latest"}}, Domain: "scalecommerce.site"},
		runtime: mock,
	}
	mock.NetworksExist[SharedNetworkName] = true

	// Seed a "running" mail container with the current expected config.
	seeded := MailContainerConfig(MailServiceConfig{Image: "axllent/mailpit:latest", Domain: "scalecommerce.site"})
	mock.Containers[MailContainerName] = seeded
	mock.ContainersExist[MailContainerName] = true
	mock.ContainersRunning[MailContainerName] = true

	if err := m.StartMail(context.Background()); err != nil {
		t.Fatalf("StartMail: %v", err)
	}

	if mock.CallCount("RemoveContainer") != 0 {
		t.Errorf("RemoveContainer called %d times, want 0", mock.CallCount("RemoveContainer"))
	}
	if mock.CallCount("CreateContainer") != 0 {
		t.Errorf("CreateContainer called %d times, want 0", mock.CallCount("CreateContainer"))
	}
}

// TestStartService_HashMismatchRecreates: container was created with
// TLSEnabled=false but current config enables TLS - startService must
// remove and recreate the container so it picks up the HTTPS labels.
// This is the exact scenario from the bug report.
func TestStartService_HashMismatchRecreates(t *testing.T) {
	mock := runtime.NewMockRuntime()
	m := &Manager{
		cfg: &config.GlobalConfig{
			Shared: config.SharedConfig{Mail: config.MailConfig{Image: "axllent/mailpit:latest"}},
			Domain: "scalecommerce.site",
			SSL:    config.SSLConfig{Enabled: true},
		},
		runtime: mock,
	}
	mock.NetworksExist[SharedNetworkName] = true

	// Seed a stale "no-TLS" container - what you'd have if the container
	// was created while SSL was off and then SSL got flipped on.
	stale := MailContainerConfig(MailServiceConfig{Image: "axllent/mailpit:latest", Domain: "scalecommerce.site", TLSEnabled: false})
	mock.Containers[MailContainerName] = stale
	mock.ContainersExist[MailContainerName] = true
	mock.ContainersRunning[MailContainerName] = true

	if err := m.StartMail(context.Background()); err != nil {
		t.Fatalf("StartMail: %v", err)
	}

	if mock.CallCount("RemoveContainer") != 1 {
		t.Errorf("RemoveContainer called %d times, want 1", mock.CallCount("RemoveContainer"))
	}
	if mock.CallCount("CreateContainer") != 1 {
		t.Errorf("CreateContainer called %d times, want 1", mock.CallCount("CreateContainer"))
	}

	// The newly created container must have the HTTPS router label.
	created := mock.Containers[MailContainerName]
	if created.Labels["traefik.http.routers.scdev-mail-https.tls"] != "true" {
		t.Error("expected recreated mail container to have HTTPS label")
	}
}

// TestStartService_ExistingStoppedMatchingHashStarts: container exists
// but is stopped, hash matches current config - plain start, no recreate.
func TestStartService_ExistingStoppedMatchingHashStarts(t *testing.T) {
	mock := runtime.NewMockRuntime()
	m := &Manager{
		cfg:     &config.GlobalConfig{Shared: config.SharedConfig{Mail: config.MailConfig{Image: "axllent/mailpit:latest"}}, Domain: "scalecommerce.site"},
		runtime: mock,
	}
	mock.NetworksExist[SharedNetworkName] = true

	seeded := MailContainerConfig(MailServiceConfig{Image: "axllent/mailpit:latest", Domain: "scalecommerce.site"})
	mock.Containers[MailContainerName] = seeded
	mock.ContainersExist[MailContainerName] = true
	mock.ContainersRunning[MailContainerName] = false

	if err := m.StartMail(context.Background()); err != nil {
		t.Fatalf("StartMail: %v", err)
	}

	if mock.CallCount("RemoveContainer") != 0 {
		t.Errorf("RemoveContainer called %d times, want 0", mock.CallCount("RemoveContainer"))
	}
	if mock.CallCount("CreateContainer") != 0 {
		t.Errorf("CreateContainer called %d times, want 0", mock.CallCount("CreateContainer"))
	}
	if !mock.CalledWith("StartContainer", MailContainerName) {
		t.Error("expected StartContainer to be called on the existing stopped container")
	}
}

// TestUnionPortSets covers the helper used by StartRouter to keep extra
// ports in the running router without triggering a recreate.
func TestUnionPortSets(t *testing.T) {
	cases := []struct {
		name   string
		a, b   []int
		expect []int
	}{
		{"both empty", nil, nil, []int{}},
		{"current superset of required", []int{3306, 5432}, []int{5432}, []int{3306, 5432}},
		{"required adds a new port", []int{3306}, []int{3306, 5432}, []int{3306, 5432}},
		{"disjoint", []int{3306}, []int{5432}, []int{3306, 5432}},
		{"duplicates within input", []int{3306, 3306, 5432}, []int{3306}, []int{3306, 5432}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := unionPortSets(tc.a, tc.b)
			if len(got) != len(tc.expect) {
				t.Fatalf("got %v, want %v", got, tc.expect)
			}
			for i := range got {
				if got[i] != tc.expect[i] {
					t.Fatalf("got %v, want %v", got, tc.expect)
				}
			}
		})
	}
}

// TestParsePortCSV covers the companion of the label → []int round-trip.
func TestParsePortCSV(t *testing.T) {
	cases := []struct {
		in     string
		expect []int
	}{
		{"", nil},
		{"3306", []int{3306}},
		{"3306,5432", []int{3306, 5432}},
		{" 3306 , 5432 ", []int{3306, 5432}},
		{"bad,3306", []int{3306}},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := parsePortCSV(tc.in)
			if len(got) != len(tc.expect) {
				t.Fatalf("got %v, want %v", got, tc.expect)
			}
			for i := range got {
				if got[i] != tc.expect[i] {
					t.Fatalf("got %v, want %v", got, tc.expect)
				}
			}
		})
	}
}
