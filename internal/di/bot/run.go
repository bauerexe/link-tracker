package bot

import (
	"context"
	"io"
	stdlog "log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/fx"
	"go.uber.org/zap"

	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
)

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

func tgBotAPIDiscard() {
	_ = tgbotapi.SetLogger(stdlog.New(io.Discard, "", 0))
}
