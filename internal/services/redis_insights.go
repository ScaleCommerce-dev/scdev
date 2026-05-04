package services

import (
	"fmt"

	"github.com/0ploy/zdev/internal/runtime"
)

// RedisInsightsContainerName is the name of the Redis Insights container
const RedisInsightsContainerName = "zdev_redis"

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
		"zdev.managed":       "true",
		"zdev.service":       "redis-insights",
		DozzleVisibilityLabel: "true",
		DozzleGroupLabel:      DozzleSharedGroup,

		// Enable Traefik routing for web UI
		"traefik.enable":         "true",
		"traefik.docker.network": SharedNetworkName,

		// HTTP router for web UI
		"traefik.http.routers.zdev-redis.rule":        fmt.Sprintf("Host(`%s`)", redisHost),
		"traefik.http.routers.zdev-redis.entrypoints": "http",
		"traefik.http.routers.zdev-redis.service":     "zdev-redis",

		// Service pointing to Redis Insights web UI port
		"traefik.http.services.zdev-redis.loadbalancer.server.port": "5540",
	}

	// Add HTTPS router if TLS is enabled
	if cfg.TLSEnabled {
		labels["traefik.http.routers.zdev-redis-https.rule"] = fmt.Sprintf("Host(`%s`)", redisHost)
		labels["traefik.http.routers.zdev-redis-https.entrypoints"] = "https"
		labels["traefik.http.routers.zdev-redis-https.tls"] = "true"
		labels["traefik.http.routers.zdev-redis-https.service"] = "zdev-redis"
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
