package cmd

import (
	"context"

	"github.com/ScaleCommerce-DEV/scdev/internal/services"
	"github.com/spf13/cobra"
)

var redisCmd = &cobra.Command{
	Use:   "redis",
	Short: "Open Redis Insights web UI",
	Long:  `Open the Redis Insights web interface in your default browser. Redis Insights provides a visual browser for Redis databases.`,
	RunE:  runRedis,
}

func init() {
	rootCmd.AddCommand(redisCmd)
}

func runRedis(cmd *cobra.Command, args []string) error {
	return openSharedServiceURL("redis", "redis.shared",
		func(ctx context.Context, mgr *services.Manager) (*services.ServiceStatus, error) {
			return mgr.RedisInsightsStatus(ctx)
		},
	)
}
