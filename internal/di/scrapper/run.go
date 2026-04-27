package scrapper

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka/outbox"
	"go.uber.org/fx"
	"go.uber.org/zap"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
)

var RunModule = fx.Options(
	fx.Invoke(
		runScrapper,
		runScheduler,
		runOutboxRelay,
	),
)

func runOutboxRelay(
	lc fx.Lifecycle,
	cfg config.KafkaConfig,
	cfgSarama *sarama.Config,
	pool *pgxpool.Pool,
	log *zap.Logger,
) {
	if !cfg.KafkaEnabled {
		log.Info("outbox relay disabled because kafka disabled")
		return
	}

	var relay *outbox.Relay
	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("starting outbox relay")

			var err error
			relay, err = outbox.NewOutboxRelay(pool, cfg, cfgSarama, log)
			if err != nil {
				return fmt.Errorf("new outbox relay: %w", err)
			}

			runCtx, c := context.WithCancel(context.Background())
			cancel = c

			go func() {
				if err = relay.Run(runCtx); err != nil {
					log.Error("outbox relay stopped", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(_ context.Context) error {
			if cancel != nil {
				cancel()
			}

			if relay != nil {
				return relay.Close()
			}

			return nil
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
