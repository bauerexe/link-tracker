package scrapper

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	controller "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
)

const defaultCacheTTL = 5 * time.Minute

var CacheModule = fx.Options(
	fx.Provide(newValkeyClient, newCache),
)

func newValkeyClient(lc fx.Lifecycle, cfg *config.ScrapperConfig) *redis.Client {
	if cfg.ValkeyAddr == "" {
		return nil
	}
	if !cfg.CacheEnabled {
		return nil
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.ValkeyAddr, Password: cfg.ValkeyPassword, DB: cfg.ValkeyDB})
	lc.Append(fx.Hook{OnStop: func(_ context.Context) error { return client.Close() }})
	return client
}

func newCache(cfg *config.ScrapperConfig, client *redis.Client) controller.Cache {
	ttl := cfg.CacheTTL
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}
	return controller.NewValkeyCache(client, ttl)
}
