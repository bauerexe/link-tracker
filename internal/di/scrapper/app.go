package scrapper

import (
	"context"
	"fmt"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	controller "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

var AppModule = fx.Options(
	fx.Provide(
		newAppContext,
		newScrapperServer,
		newScrapperApp,
		newBotNotifier,
		newCheckers,
		newScheduler,
	),
)

func newAppContext(lc fx.Lifecycle, log *zap.Logger) context.Context {
	appCtx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			log.Info("stopping app context")
			cancel()
			return nil
		},
	})

	return appCtx
}

func newScrapperServer(
	log *zap.Logger,
	chatRepo scrapperapp.ChatRepository,
	linkRepo scrapperapp.LinkRepository,
	trackLinkService *scrapperapp.TrackLinkService,
) pbv1.ScrapperServer {
	return controller.New(log, chatRepo, linkRepo, trackLinkService)
}
func newScrapperApp(
	server pbv1.ScrapperServer,
	log *zap.Logger,
	cfg *config.ScrapperConfig,
) *scrapperapp.Scrapper {
	sa := scrapperapp.New(server, log, cfg)
	return &sa
}

func newBotNotifier(client pbv1.BotClient, producer *kafka.ProducerScrapperToBot, log *zap.Logger, cfgKafka config.KafkaConfig) scrapperapp.BotNotifier {
	if cfgKafka.KafkaEnabled {
		log.Info("starting bot notifier by producer kafka")
		return scrapperapp.NewKafkaBotNotifier(producer, log)
	}
	log.Info("starting bot notifier by gRPC")
	return scrapperapp.NewGRPCBotNotifier(client, log)

}

func newCheckers(gc github.Client, sc stackoverflow.Client, gr github.Repository, log *zap.Logger,
) []scrapperapp.Checker {
	gitChecker, err := github.NewChecker(gc, gr, log)
	if err != nil {
		log.Error("error creating github checker", zap.Error(err))
	}
	stackChecker, err := stackoverflow.NewChecker(sc, log)
	if err != nil {
		log.Error("error creating stackoverflow checker", zap.Error(err))
	}
	return []scrapperapp.Checker{
		gitChecker,
		stackChecker,
	}
}

func newScheduler(
	linkRepo scrapperapp.LinkRepository,
	notifier scrapperapp.BotNotifier,
	checkers []scrapperapp.Checker,
	log *zap.Logger,
	cfg *config.ScrapperConfig,
) (*scrapperapp.Scheduler, error) {
	interval := time.Duration(cfg.MinutesIntervalCheck) * time.Minute

	s, err := scrapperapp.NewScheduler(&scrapperapp.Scheduler{
		Links:       linkRepo,
		Notifier:    notifier,
		Checkers:    checkers,
		Log:         log,
		Interval:    interval,
		BatchSize:   cfg.BatchSize,
		WorkerCount: cfg.WorkerCount,
	})
	if err != nil {
		return nil, fmt.Errorf("new scheduler: %w", err)
	}

	return s, nil
}
