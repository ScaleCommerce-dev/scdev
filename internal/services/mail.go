package services

import (
	"fmt"

	"github.com/0ploy/zdev/internal/runtime"
)

// MailContainerName is the name of the Mailpit container
const MailContainerName = "zdev_mail"

// MailServiceConfig holds configuration for the mail container
type MailServiceConfig struct {
	Image      string
	Domain     string
	TLSEnabled bool
}

// MailContainerConfig returns the container configuration for Mailpit
func MailContainerConfig(cfg MailServiceConfig) runtime.ContainerConfig {
	mailHost := fmt.Sprintf("mail.shared.%s", cfg.Domain)

	labels := map[string]string{
		"zdev.managed":       "true",
		"zdev.service":       "mail",
		DozzleVisibilityLabel: "true",
		DozzleGroupLabel:      DozzleSharedGroup,

		// Enable Traefik routing for web UI
		"traefik.enable":         "true",
		"traefik.docker.network": SharedNetworkName,

		// HTTP router for web UI
		"traefik.http.routers.zdev-mail.rule":        fmt.Sprintf("Host(`%s`)", mailHost),
		"traefik.http.routers.zdev-mail.entrypoints": "http",
		"traefik.http.routers.zdev-mail.service":     "zdev-mail",

		// Service pointing to Mailpit web UI port
		"traefik.http.services.zdev-mail.loadbalancer.server.port": "8025",
	}

	// Add HTTPS router if TLS is enabled
	if cfg.TLSEnabled {
		labels["traefik.http.routers.zdev-mail-https.rule"] = fmt.Sprintf("Host(`%s`)", mailHost)
		labels["traefik.http.routers.zdev-mail-https.entrypoints"] = "https"
		labels["traefik.http.routers.zdev-mail-https.tls"] = "true"
		labels["traefik.http.routers.zdev-mail-https.service"] = "zdev-mail"
	}

	out := runtime.ContainerConfig{
		Name:        MailContainerName,
		Image:       cfg.Image,
		NetworkName: SharedNetworkName,
		Aliases:     []string{"mail"},
		Labels:      labels,
		// No ports exposed directly - SMTP (1025) accessed via network alias,
		// Web UI (8025) accessed via Traefik routing
	}
	runtime.StampConfigHash(&out)
	return out
}
