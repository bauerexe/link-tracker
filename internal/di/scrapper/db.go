package scrapper

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/db"
)

var DBModule = fx.Options(
	fx.Provide(
		newPostgresPool,
	),
)

func newPostgresPool(
	lc fx.Lifecycle,
	cfg *config.ScrapperConfig,
	log *zap.Logger,
) (*pgxpool.Pool, error) {
	pool, err := db.NewPostgresPool(context.Background(), cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			log.Info("closing postgres pool")
			pool.Close()
			return nil
		},
	})

	return pool, nil
}
