package scrapper

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
)

var RunModule = fx.Options(
	fx.Invoke(
		runScrapper,
		runScheduler,
		runProducer,
	),
)

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

func runProducer(
	lc fx.Lifecycle,
	cfg config.KafkaConfig,
	producer *kafka.ProducerScrapperToBot,
	log *zap.Logger,
) {
	if !cfg.KafkaEnabled {
		log.Info("kafka producer disabled")
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("starting kafka producer")

			go func() {
				if err := producer.Run(); err != nil {
					log.Error("kafka producer stopped", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(producer.Messages)
			return nil
		},
	})
}
