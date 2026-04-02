package scrapper

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/db"
)

var RunModule = fx.Options(
	fx.Invoke(
		runMigrations,
		runScrapper,
		runScheduler,
	),
)

func runMigrations(lc fx.Lifecycle, cfg *config.ScrapperConfig) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			return db.RunMigrations(cfg.PostgresDSN, cfg.MigrationsPath)
		},
	})
}

func runScrapper(
	appCtx context.Context,
	lc fx.Lifecycle,
	scrapper *scrapperapp.Scrapper,
	log *zap.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("start scrapper")
			go scrapper.Run(appCtx)
			return nil
		},
	})
}

func runScheduler(
	appCtx context.Context,
	lc fx.Lifecycle,
	scheduler *scrapperapp.Scheduler,
	log *zap.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("start scheduler")
			go scheduler.Run(appCtx)
			return nil
		},
	})
}
