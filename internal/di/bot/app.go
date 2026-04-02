package bot

import (
	"context"
	"fmt"

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
	),
	fx.Invoke(
		tgBotAPIDiscard,
		runBot,
	),
)

func newContext() context.Context {
	return context.Background()
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

func newBotServer(log *zap.Logger, repo botapp.BotGateway) pbv1.BotServer {
	return botcontroller.New(log.With(zap.String("layer", "controller")), repo)
}

func newBotUsecase(
	cfg config.BotConfig,
	repo botapp.BotGateway,
	server pbv1.BotServer,
	router *botapp.BotDispatcher,
	log *zap.Logger,
) (*botapp.Bot, error) {
	bot, err := botapp.NewBot(
		cfg.TokenTGBot,
		repo,
		server,
		router,
		log.With(zap.String("layer", "application")).Named("usecase.bot"),
		&cfg,
	)
	if err != nil {
		return nil, fmt.Errorf("create bot usecase: %w", err)
	}

	return bot, nil
}
