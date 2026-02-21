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

	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/config"
	botrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/bot"
)

func main() {
	app := fx.New(
		fx.Provide(
			newZap,
			newConfig,
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

func newRouter() *botapp.BotDispatcher {
	return botapp.NewBotDispatcher(map[domain.Command]domain.Handler{
		botapp.CommandStart: botapp.NewStartHandler(),
		botapp.CommandHelp:  botapp.NewHelpHandler(),
	})
}

func newBotRepo(cfg config.BotConfig,
	log *zap.Logger) (botapp.BotRepository, error) {
	return botrepo.New(cfg.TokenTGBot, log.With(zap.String("layer", "infrastructure")).Named("telegram"))
}

func newBotUsecase(cfg config.BotConfig,
	repo botapp.BotRepository,
	router *botapp.BotDispatcher,
	log *zap.Logger) (*botapp.Bot, error) {
	return botapp.NewBot(cfg.TokenTGBot, repo, router, log.With(zap.String("layer", "application")).Named("usecase.bot"))
}

func runBot(lc fx.Lifecycle, bot *botapp.Bot, log *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("starting bot")
			go func() {
				if err := bot.Run(context.Background()); err != nil {
					log.Error("bot stopped with error", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("stopping bot")
			_ = log.Sync()
			return nil
		},
	})
}

func tgBotApiDiscard() {
	_ = tgbotapi.SetLogger(stdlog.New(io.Discard, "", 0))
}
