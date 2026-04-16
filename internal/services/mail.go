package services

import (
	"fmt"

	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
)

// MailContainerName is the name of the Mailpit container
const MailContainerName = "scdev_mail"

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
		"scdev.managed": "true",
		"scdev.service": "mail",

		// Enable Traefik routing for web UI
		"traefik.enable":        "true",
		"traefik.docker.network": SharedNetworkName,

		// HTTP router for web UI
		"traefik.http.routers.scdev-mail.rule":        fmt.Sprintf("Host(`%s`)", mailHost),
		"traefik.http.routers.scdev-mail.entrypoints": "http",
		"traefik.http.routers.scdev-mail.service":     "scdev-mail",

		// Service pointing to Mailpit web UI port
		"traefik.http.services.scdev-mail.loadbalancer.server.port": "8025",
	}

	// Add HTTPS router if TLS is enabled
	if cfg.TLSEnabled {
		labels["traefik.http.routers.scdev-mail-https.rule"] = fmt.Sprintf("Host(`%s`)", mailHost)
		labels["traefik.http.routers.scdev-mail-https.entrypoints"] = "https"
		labels["traefik.http.routers.scdev-mail-https.tls"] = "true"
		labels["traefik.http.routers.scdev-mail-https.service"] = "scdev-mail"
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
