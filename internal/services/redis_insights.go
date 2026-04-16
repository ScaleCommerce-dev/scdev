package services

import (
	"fmt"

	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
)

// RedisInsightsContainerName is the name of the Redis Insights container
const RedisInsightsContainerName = "scdev_redis"

// RedisInsightsServiceConfig holds configuration for the Redis Insights container
type RedisInsightsServiceConfig struct {
	Image      string
	Domain     string
	TLSEnabled bool
}

// RedisInsightsContainerConfig returns the container configuration for Redis Insights
func RedisInsightsContainerConfig(cfg RedisInsightsServiceConfig) runtime.ContainerConfig {
	redisHost := fmt.Sprintf("redis.shared.%s", cfg.Domain)

	labels := map[string]string{
		"scdev.managed": "true",
		"scdev.service": "redis-insights",

		// Enable Traefik routing for web UI
		"traefik.enable":         "true",
		"traefik.docker.network": SharedNetworkName,

		// HTTP router for web UI
		"traefik.http.routers.scdev-redis.rule":        fmt.Sprintf("Host(`%s`)", redisHost),
		"traefik.http.routers.scdev-redis.entrypoints": "http",
		"traefik.http.routers.scdev-redis.service":     "scdev-redis",

		// Service pointing to Redis Insights web UI port
		"traefik.http.services.scdev-redis.loadbalancer.server.port": "5540",
	}

	// Add HTTPS router if TLS is enabled
	if cfg.TLSEnabled {
		labels["traefik.http.routers.scdev-redis-https.rule"] = fmt.Sprintf("Host(`%s`)", redisHost)
		labels["traefik.http.routers.scdev-redis-https.entrypoints"] = "https"
		labels["traefik.http.routers.scdev-redis-https.tls"] = "true"
		labels["traefik.http.routers.scdev-redis-https.service"] = "scdev-redis"
	}

	out := runtime.ContainerConfig{
		Name:        RedisInsightsContainerName,
		Image:       cfg.Image,
		NetworkName: SharedNetworkName,
		Aliases:     []string{"redis-insights"},
		Labels:      labels,
		// No ports exposed directly - Web UI (5540) accessed via Traefik routing
	}
	runtime.StampConfigHash(&out)
	return out
}
