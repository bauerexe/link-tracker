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

const timeoutSec = 60

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

	updates, err := b.botRepository.GetMessages(ctx, timeoutSec)
	if err != nil {
		b.log.Error("failed to get messages", zap.Error(err))
		return err
	}

	b.log.Info("bot run")

	for {
		select {
		case <-ctx.Done():
			b.log.Info("ctx done", zap.Error(ctx.Err()))
			return ctx.Err()

		case upd, ok := <-updates:
			cont, err := b.handleIncomingMessage(upd, ok)
			if err != nil {
				return err
			}
			if !cont {
				return nil
			}
		}
	}
}

func (b *Bot) handleIncomingMessage(upd domain.Message, ok bool) (bool, error) {
	if !ok {
		b.log.Info("updates channel closed")
		return false, nil
	}

	logger := b.log.With(
		zap.String("msg", upd.Text),
		zap.Int("msgId", upd.MessageID),
		zap.Int64("chatId", upd.ChatID),
	)
	logger.Info("bot got message")

	text := strings.TrimSpace(upd.Text)
	if text == "" {
		return true, nil
	}

	if err := b.processMessage(logger, upd, text); err != nil {
		return false, err
	}

	return true, nil
}

func (b *Bot) processMessage(logger *zap.Logger, upd domain.Message, text string) error {
	cmd, args := parseCommand(text)

	replyText, err := b.router.Dispatch(upd, domain.Command(cmd), args)
	if err != nil {
		logger.Error(
			"error invalid command",
			zap.String("command", cmd),
			zap.Error(err),
		)
		return err
	}

	logger.Info("bot dispatched message to reply", zap.String("reply", replyText))

	if err := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, replyText); err != nil {
		logger.Error("error send message", zap.Error(err))
		return err
	}

	logger.Info("bot sent reply message to user", zap.String("reply", replyText))
	return nil
}

func parseCommand(text string) (cmd, args string) {
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
