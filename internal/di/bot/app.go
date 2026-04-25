package bot

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"

	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	botcontroller "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

var AppModule = fx.Options(
	fx.Provide(
		newContext,
		newRouter,
		newBotRepo,
		newBotServer,
		newBotUsecase,
		NewConsumer,
	),
	fx.Invoke(
		tgBotAPIDiscard,
		runBot,
		runConsumer,
	),
)

func newContext() context.Context {
	return context.Background()
}

func runConsumer(
	lc fx.Lifecycle,
	cfg config.KafkaConfig,
	consumer *kafka.ConsumerBotFromScrapper,
	log *zap.Logger,
) {
	if !cfg.KafkaEnabled {
		log.Info("kafka consumer disabled")
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("starting kafka consumer")

			go func() {
				if err := consumer.Run(ctx); err != nil {
					log.Error("kafka consumer stopped", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return consumer.Close()
		},
	})
}

func newRouter(ctx context.Context, client pbv1.ScrapperClient) *botapp.BotDispatcher {
	return botapp.NewBotDispatcher(map[domain.Command]domain.Handler{
		botapp.CommandStart:   handlers.NewStartHandler(ctx, client),
		botapp.CommandHelp:    handlers.NewHelpHandler(),
		botapp.CommandTrack:   handlers.NewTrackHandler(ctx, client),
		botapp.CommandUntrack: handlers.NewUntrackHandler(ctx, client),
		botapp.CommandList:    handlers.NewListHandler(ctx, client),
	})
}

func NewConsumer(cfgKafka config.KafkaConfig, cfgSarama *sarama.Config, log *zap.Logger, bot botapp.BotGateway) (*kafka.ConsumerBotFromScrapper, error) {
	c, err := kafka.NewConsumer(cfgKafka, cfgSarama, log, bot)
	if err != nil {
		return nil, fmt.Errorf("error creating consumer from kafka: %w", err)
	}
	return c, nil
}

func newBotServer(log *zap.Logger, repo botapp.BotGateway) pbv1.BotServer {
	return botcontroller.New(log.With(zap.String("layer", "controller")), repo)
}

func newBotUsecase(
	cfg config.BotConfig,
	cfgKafka config.KafkaConfig,
	repo botapp.BotGateway,
	router *botapp.BotDispatcher,
	server pbv1.BotServer,
	log *zap.Logger,
) (*botapp.Bot, error) {
	if cfgKafka.KafkaEnabled {
		server = nil
	}

	return botapp.NewBot(
		cfg.TokenTGBot,
		repo,
		server,
		router,
		log,
		&cfg,
	)
}
