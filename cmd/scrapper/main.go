package main

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	controller "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	repository "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/scrapper/inmemory"
)

func main() {
	fx.New(
		fx.Provide(
			newZap,
			newScrapper,
		),
		fx.Invoke(runScrapper),
	).Run()
}

func newZap() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	log, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	log = log.Named("scrapper").With(zap.String("service", "scrapper"))
	return log, nil
}

func newScrapper(log *zap.Logger) scrapperapp.Scrapper {
	server := controller.New(log, repository.NewChatRepository(), repository.NewLinkRepository())
	return scrapperapp.New(server, log)
}

func runScrapper(lc fx.Lifecycle, scrapper scrapperapp.Scrapper, log *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("start scrapper")
			go func() {
				scrapper.Run(ctx)
			}()
			return nil
		},
		OnStop: nil,
	})
}
