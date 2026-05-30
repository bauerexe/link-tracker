package scrapper

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/observability"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var MetricsModule = fx.Options(
	fx.Invoke(registerTrackedLinksCollector),
)

func registerTrackedLinksCollector(pool *pgxpool.Pool, log *zap.Logger) error {
	if err := observability.RegisterTrackedLinksCollector(pool); err != nil {
		log.Error("register tracked links collector failed", zap.Error(err))
		return fmt.Errorf("register tracked links collector: %w", err)
	}

	return nil
}
