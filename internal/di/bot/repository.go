package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	botrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/gateway/bot"
)

func newBotRepo(cfg config.BotConfig, log *zap.Logger) (botapp.BotGateway, error) {
	if cfg.TelegramDisabled {
		log.Info("telegram disabled by BOT_DISABLE_TELEGRAM; using noop gateway")
		return botrepo.NewDummy(
			log.With(zap.String("layer", "infrastructure")).Named("telegram.noop"),
		), nil
	}

	repo, err := botrepo.New(
		cfg.TokenTGBot,
		log.With(zap.String("layer", "infrastructure")).Named("telegram"),
		[]tgbotapi.BotCommand{
			{Command: "help", Description: "помощь"},
			{Command: "start", Description: "старт"},
			{Command: "track", Description: "отслеживание ссылки"},
			{Command: "untrack", Description: "прекращение отслеживания"},
			{Command: "list", Description: "список отслеживаемых ссылок"},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create bot repository: %w", err)
	}

	return repo, nil
}
