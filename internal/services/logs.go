package services

import (
	"fmt"

	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
)

// LogsContainerName is the name of the Dozzle log viewer container
const LogsContainerName = "scdev_logs"

// LogsDataVolumeName is the named volume backing Dozzle's /data directory
// (notification settings, saved searches, user state).
const LogsDataVolumeName = "scdev_logs_data"

// LogsServiceConfig holds configuration for the Dozzle container
type LogsServiceConfig struct {
	Image      string
	Domain     string
	TLSEnabled bool
}

// LogsContainerConfig returns the container configuration for Dozzle.
// Dozzle reads container info from the Docker socket; project containers
// stamp dev.dozzle.group=<project> in buildContainerConfig so they cluster
// per-project in the UI.
func LogsContainerConfig(cfg LogsServiceConfig) runtime.ContainerConfig {
	logsHost := fmt.Sprintf("logs.shared.%s", cfg.Domain)

	labels := map[string]string{
		"scdev.managed":       "true",
		"scdev.service":       "logs",
		DozzleVisibilityLabel: "true",
		DozzleGroupLabel:      DozzleSharedGroup,

		// Enable Traefik routing for web UI
		"traefik.enable":         "true",
		"traefik.docker.network": SharedNetworkName,

		// HTTP router for web UI
		"traefik.http.routers.scdev-logs.rule":        fmt.Sprintf("Host(`%s`)", logsHost),
		"traefik.http.routers.scdev-logs.entrypoints": "http",
		"traefik.http.routers.scdev-logs.service":     "scdev-logs",

		// Service pointing to Dozzle web UI port
		"traefik.http.services.scdev-logs.loadbalancer.server.port": "8080",
	}

	// Add HTTPS router if TLS is enabled
	if cfg.TLSEnabled {
		labels["traefik.http.routers.scdev-logs-https.rule"] = fmt.Sprintf("Host(`%s`)", logsHost)
		labels["traefik.http.routers.scdev-logs-https.entrypoints"] = "https"
		labels["traefik.http.routers.scdev-logs-https.tls"] = "true"
		labels["traefik.http.routers.scdev-logs-https.service"] = "scdev-logs"
	}

	out := runtime.ContainerConfig{
		Name:        LogsContainerName,
		Image:       cfg.Image,
		NetworkName: SharedNetworkName,
		Aliases:     []string{"logs"},
		Labels:      labels,
		Env: map[string]string{
			"DOZZLE_NO_ANALYTICS": "true",
			// Restrict Dozzle to opted-in containers. Shared services always
			// stamp scdev.shared.logs=true; project containers only get the
			// label when their config sets shared.logs: true (see
			// internal/project/project.go buildContainerConfig). Other
			// scdev-managed containers and unrelated containers are hidden.
			"DOZZLE_FILTER": "label=" + DozzleVisibilityLabel + "=true",
			// Allow opening a shell into containers from the Dozzle UI.
			"DOZZLE_ENABLE_SHELL": "true",
		},
		Volumes: []runtime.VolumeMount{
			{
				Source:   "/var/run/docker.sock",
				Target:   "/var/run/docker.sock",
				ReadOnly: true,
			},
			// Persist Dozzle's own state (notification settings, user data,
			// saved searches) across container recreates. Named volume; Docker
			// auto-creates it on first start.
			{
				Source: LogsDataVolumeName,
				Target: "/data",
			},
		},
	}
	runtime.StampConfigHash(&out)
	return out
}
