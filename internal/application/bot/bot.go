package botapp

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/zap"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

// Bot - use case
type Bot struct {
	botRepository BotRepository
	router        *BotDispatcher
	log           *zap.Logger
}

func NewBot(token string, botRepository BotRepository, router *BotDispatcher, log *zap.Logger) (*Bot, error) {
	log = log.Named("application")
	log = log.With(zap.String("pkg", "botapp"))

	if token == "" {
		log.Error("error empty telegram token")
		return nil, errors.New("empty telegram token")
	}
	if router == nil {
		log.Error("error nil router")
		return nil, errors.New("nil router")
	}

	return &Bot{botRepository: botRepository, router: router, log: log}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	updates, err := b.botRepository.GetMessages(ctx, 60)
	if err != nil {
		b.log.Error(
			"error invalid command",
			zap.String("error", err.Error()),
		)
		return err
	}
	b.log.Info("bot run")
	for {
		select {
		case <-ctx.Done():
			b.log.Info("ctx done", zap.String("err", ctx.Err().Error()))
			return ctx.Err()

		case upd, ok := <-updates:
			b.log = b.log.With(
				zap.String("msg", upd.Text),
				zap.Int("msgId", upd.MessageID),
				zap.Int64("chatId", upd.ChatID))
			b.log.Info("bot got message")
			if !ok {
				return nil
			}

			if strings.TrimSpace(upd.Text) == "" {
				continue
			}

			chatID := upd.ChatID
			text := strings.TrimSpace(upd.Text)

			cmd, args := parseCommand(text)

			replyText, derr := b.router.Dispatch(chatID, domain.Command(cmd), args)
			if derr != nil {
				b.log.Error(
					"error invalid command",
					zap.String("command", cmd),
					zap.String("error", derr.Error()),
				)
				return derr
			}
			b.log.Info("bot dispatched message to reply",
				zap.String("reply", replyText),
			)
			if serr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, replyText); serr != nil {
				b.log.Error(
					"error send message",
					zap.String("error", serr.Error()),
				)
				return serr
			}
			b.log.Info("bot sent reply message to user",
				zap.String("reply", replyText),
			)
		}
	}
}

func parseCommand(text string) (cmd string, args string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", ""
	}
	cmd = parts[0]
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}
	return cmd, args
}
