package main

import (
	"context"
	"io"
	stdlog "log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/afero"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/config"
	botrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/gateway/bot"
	botcontroller "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

func main() {
	app := fx.New(
		fx.Provide(
			newZap,
			newCtx,
			newConfig,
			newScrapperConn,
			newScrapperClient,
			newBotServer,
			newRouter,
			newBotRepo,
			newBotUsecase,
		),
		fx.Invoke(
			tgBotApiDiscard,
			runBot,
		),
	)

	app.Run()
}

func newConfig(log *zap.Logger) (config.BotConfig, error) {
	fs := afero.NewOsFs()
	cfg, err := config.NewBotConfig(fs)
	if err != nil {
		return config.BotConfig{}, err
	}
	log.Info("init config")
	return cfg, nil
}

func newZap() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	log, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	log = log.Named("bot").With(zap.String("service", "bot"))
	return log, nil
}

func newCtx() context.Context {
	return context.Background()
}

func newScrapperConn(lc fx.Lifecycle, cfg config.BotConfig, log *zap.Logger) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		cfg.ScrapperAddrGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	log.Info("connected to scrapper", zap.String("addr", cfg.ScrapperAddrGRPC))

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			log.Info("closing scrapper connection")
			return conn.Close()
		},
	})

	return conn, nil
}

func newScrapperClient(conn *grpc.ClientConn) pbv1.ScrapperClient {
	return pbv1.NewScrapperClient(conn)
}

func newRouter(ctx context.Context, client pbv1.ScrapperClient) *botapp.BotDispatcher {
	return botapp.NewBotDispatcher(map[domain.Command]domain.Handler{
		botapp.CommandStart:   handlers.NewStartHandler(ctx, client),
		botapp.CommandHelp:    handlers.NewHelpHandler(),
		botapp.CommandTrack:   handlers.NewTrackHandler(ctx, client),
		botapp.CommandUntrack: handlers.NewUntrackHandler(ctx, client),
		botapp.CommandList:    handlers.NewListHandler(ctx, client),
	},
	)
}

func newBotServer(log *zap.Logger, repo botapp.BotGateway) pbv1.BotServer {
	return botcontroller.New(log.With(zap.String("layer", "controller")), repo)
}

func newBotRepo(cfg config.BotConfig,
	log *zap.Logger,
) (botapp.BotGateway, error) {
	if cfg.TelegramDisabled {
		log.Info("telegram disabled by BOT_DISABLE_TELEGRAM; using noop gateway")
		return botrepo.NewDummy(log.With(zap.String("layer", "infrastructure")).Named("telegram.noop")), nil
	}
	return botrepo.New(cfg.TokenTGBot, log.With(zap.String("layer", "infrastructure")).Named("telegram"),
		[]tgbotapi.BotCommand{
			{Command: "help", Description: "помощь"},
			{Command: "start", Description: "старт"},
			{Command: "track", Description: "отслеживание ссылки"},
			{Command: "untrack", Description: "прекращение отслеживания"},
			{Command: "list", Description: "список отслеживаемых ссылок"},
		})
}

func newBotUsecase(cfg config.BotConfig,
	repo botapp.BotGateway,
	server pbv1.BotServer,
	router *botapp.BotDispatcher,
	log *zap.Logger,
) (*botapp.Bot, error) {
	return botapp.NewBot(cfg.TokenTGBot, repo, server, router,
		log.With(zap.String("layer", "application")).Named("usecase.bot"), &cfg)
}

func runBot(lc fx.Lifecycle, bot *botapp.Bot, log *zap.Logger) {
	var cancel context.CancelFunc
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("starting bot")

			botCtx, c := context.WithCancel(context.Background())
			cancel = c
			go func() {
				if err := bot.Run(botCtx); err != nil {
					if botCtx.Err() != nil {
						log.Info("bot stopped", zap.String("reason", botCtx.Err().Error()))
						return
					}
					log.Error("bot stopped with error", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("stopping bot")

			if cancel != nil {
				cancel()
			}

			done := make(chan struct{})
			go func() {
				_ = log.Sync()
				close(done)
			}()

			select {
			case <-done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}

func tgBotApiDiscard() {
	_ = tgbotapi.SetLogger(stdlog.New(io.Discard, "", 0))
}
