package main

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/config"
	botrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/bot"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	cfgZap := zap.NewProductionConfig()
	cfgZap.EncoderConfig.TimeKey = "ts"
	cfgZap.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	log, _ := cfgZap.Build()
	log = log.Named("main")
	log = log.With(zap.String("pkg", "cmd"))

	defer func(log *zap.Logger) {
		_ = log.Sync()
	}(log)

	cfg, err := config.NewBotConfig()
	if err != nil {
		log.Panic("failed to parse config")
	}
	log.Info("config parsed: OK")

	repo, err := botrepo.New(cfg.TokenTGBot, log)
	if err != nil {
		log.Panic(err.Error())
	}
	log.Info("repository init: OK")

	router := botapp.NewBotDispatcher(map[domain.Command]domain.Handler{
		botapp.CommandStart: botapp.NewStartHandler(),
		botapp.CommandHelp:  botapp.NewHelpHandler(),
	})

	bot, err := botapp.NewBot(cfg.TokenTGBot, repo, router, log)
	if err != nil {
		log.Panic(err.Error())
	}
	if bot == nil {
		log.Panic("bot is nil")
	}
	log.Info("bot init: OK")
	if err := bot.Run(context.Background()); err != nil {
		log.Panic(err.Error())
	}
}
