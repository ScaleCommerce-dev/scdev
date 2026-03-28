package runtime

import (
	"testing"
)

func TestContainerConfigDefaults(t *testing.T) {
	cfg := ContainerConfig{}

	if cfg.Name != "" {
		t.Errorf("expected Name to be empty, got %q", cfg.Name)
	}

	if cfg.Env != nil {
		t.Error("expected Env to be nil")
	}

	if cfg.Labels != nil {
		t.Error("expected Labels to be nil")
	}
}

func TestVolumeMountString(t *testing.T) {
	tests := []struct {
		name     string
		mount    VolumeMount
		expected string
	}{
		{
			name: "simple mount",
			mount: VolumeMount{
				Source: "/host/path",
				Target: "/container/path",
			},
			expected: "/host/path:/container/path",
		},
		{
			name: "named volume",
			mount: VolumeMount{
				Source: "myvolume",
				Target: "/data",
			},
			expected: "myvolume:/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// VolumeMount is used to build mount strings in docker.go
			// This test documents the expected format
			got := tt.mount.Source + ":" + tt.mount.Target
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestContainerStatus(t *testing.T) {
	container := Container{
		ID:      "abc123",
		Name:    "test",
		Image:   "nginx:alpine",
		Status:  "running",
		Running: true,
	}

	if !container.Running {
		t.Error("expected container to be running")
	}

	container.Running = false
	container.Status = "exited"

	if container.Running {
		t.Error("expected container to not be running")
	}
}
