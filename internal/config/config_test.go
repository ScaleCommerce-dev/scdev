package config

import (
	"testing"
)

func TestProjectConfigDefaults(t *testing.T) {
	// Test that zero values are sensible
	cfg := ProjectConfig{}

	if cfg.Version != 0 {
		t.Errorf("expected Version to be 0, got %d", cfg.Version)
	}

	if cfg.Services != nil {
		t.Error("expected Services to be nil")
	}

	if cfg.Volumes != nil {
		t.Error("expected Volumes to be nil")
	}
}

func TestServiceConfigFields(t *testing.T) {
	svc := ServiceConfig{
		Image:      "node:20-alpine",
		WorkingDir: "/app",
		Command:    "npm run dev",
		Environment: map[string]string{
			"NODE_ENV": "development",
		},
	}

	if svc.Image != "node:20-alpine" {
		t.Errorf("expected Image to be 'node:20-alpine', got '%s'", svc.Image)
	}

	if svc.Environment["NODE_ENV"] != "development" {
		t.Errorf("expected NODE_ENV to be 'development', got '%s'", svc.Environment["NODE_ENV"])
	}
}
