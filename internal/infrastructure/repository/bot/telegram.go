package bot_repo

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

// BotRepository - repository for work with tg bot api
type BotRepository struct {
	api *tgbotapi.BotAPI
	log *zap.Logger
}

// New - return inited botapp.BotRepository
func New(token string, log *zap.Logger) (botapp.BotRepository, error) {
	log = log.Named("infrastructure.telegram")
	log = log.With(zap.String("pkg", log.Name()))
	httpClient := &http.Client{Timeout: 60 * time.Second}
	api, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, httpClient)
	api.Debug = false

	if err != nil {
		log.Error("error connect to bot api")
		return nil, err
	}

	cfg := tgbotapi.NewSetMyCommands(tgbotapi.BotCommand{Command: "help", Description: "помощь"}, tgbotapi.BotCommand{Command: "start", Description: "старт"})
	_, err = api.Request(cfg)
	if err != nil {
		log.Error("error set commands to bot")
		return nil, err
	}

	api.Debug = true

	return &BotRepository{api: api, log: log}, nil
}

func (r *BotRepository) GetMessages(ctx context.Context, timeoutSec int) (<-chan domain.Message, error) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = timeoutSec
	r.log.Info("start receiving updates", zap.Int("timeout_sec", timeoutSec))

	tgCh := r.api.GetUpdatesChan(u)

	out := make(chan domain.Message)

	go func() {
		defer close(out)
		defer func() {
			r.api.StopReceivingUpdates()
			r.log.Info("stopped receiving updates")
		}()

		for {
			select {
			case <-ctx.Done():
				r.log.Info("context cancelled", zap.Error(ctx.Err()))
				return

			case upd, ok := <-tgCh:
				if !ok {
					r.log.Warn("telegram updates channel closed")
					return
				}
				if upd.Message == nil {
					continue
				}

				msg := domain.Message{
					ChatID:    upd.Message.Chat.ID,
					Text:      upd.Message.Text,
					MessageID: upd.Message.MessageID,
				}

				r.log.Debug("message received",
					zap.Int64("chat_id", msg.ChatID),
					zap.Int("message_id", msg.MessageID),
				)

				select {
				case out <- msg:
				case <-ctx.Done():
					r.log.Info("context cancelled while forwarding message", zap.Error(ctx.Err()))
					return
				}
			}
		}
	}()
	return out, nil
}

func (r *BotRepository) SendMessage(chatID int64, replyToMessageID int, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyToMessageID

	_, err := r.api.Send(msg)
	if err != nil {
		r.log.Error("send message failed",
			zap.Int64("chat_id", chatID),
			zap.Int("reply_to", replyToMessageID),
			zap.Error(err),
		)
		return err
	}

	r.log.Debug("message sent",
		zap.Int64("chat_id", chatID),
		zap.Int("reply_to", replyToMessageID),
	)
	return nil
}
