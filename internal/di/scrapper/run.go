package scrapper

import (
	"context"
	"errors"
	"fmt"
	"sync"

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

	var (
		relay  *outbox.Relay
		cancel context.CancelFunc
		wg     sync.WaitGroup
	)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("starting outbox relay")

			r, err := outbox.NewOutboxRelay(pool, cfg, cfgSarama, log)
			if err != nil {
				return fmt.Errorf("new outbox relay: %w", err)
			}

			relay = r

			runCtx, c := context.WithCancel(context.Background())
			cancel = c

			wg.Add(1)
			go func() {
				defer wg.Done()

				if err = relay.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
					log.Error("outbox relay stopped with error", zap.Error(err))
				}

				log.Info("outbox relay stopped")
			}()

			return nil
		},

		OnStop: func(ctx context.Context) error {
			log.Info("stopping outbox relay")

			if cancel != nil {
				cancel()
			}

			done := make(chan struct{})

			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-ctx.Done():
				return fmt.Errorf("stop outbox relay: %w", ctx.Err())
			}

			if relay != nil {
				if err := relay.Close(); err != nil {
					return fmt.Errorf("close outbox relay: %w", err)
				}
			}

			log.Info("outbox relay stopped successfully")

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
