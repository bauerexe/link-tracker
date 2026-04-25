package kafka

import (
	"fmt"
	"strconv"

	"github.com/IBM/sarama"
	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type Handler struct {
	log *zap.Logger
	bot botapp.BotGateway
}

func NewHandler(log *zap.Logger, bot botapp.BotGateway) *Handler {
	if log != nil {
		log = log.Named("kafka consumer handler")
	}

	return &Handler{log: log, bot: bot}
}

// Setup вызывается при инициализации сессии (перед чтением сообщений).
func (h *Handler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup вызывается после завершения сессии (можно освободить ресурсы).
func (h *Handler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim — основной метод обработки. Вызывается в отдельной горутине для каждой partition.
func (h *Handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// claim.Messages() возвращает канал, в который поступают сообщения из этой partition
	for message := range claim.Messages() {
		var msg pbv1.UpdateLinkRequest

		if err := proto.Unmarshal(message.Value, &msg); err != nil {
			h.log.Error("failed to unmarshal protobuf",
				zap.Error(err),
				zap.Int32("partition", message.Partition),
				zap.Int64("offset", message.Offset),
			)
			session.MarkMessage(message, "")
			continue
		}
		if len(msg.GetTgChatIds()) == 0 {
			h.log.Debug("tgChatIds is empty",
				zap.Int64("id", msg.GetId()),
				zap.String("url", msg.GetUrl()),
			)

			session.MarkMessage(message, "")
			continue
		}
		text := formatUpdate(&msg)

		var failed []string

		for _, chatID := range msg.GetTgChatIds() {
			if err := h.bot.SendMessage(chatID, 0, text); err != nil {
				h.log.Error("failed to send telegram message",
					zap.Error(err),
					zap.Int64("chat_id", chatID),
					zap.Int64("id", msg.GetId()),
					zap.String("url", msg.GetUrl()),
				)

				failed = append(failed, strconv.FormatInt(chatID, 10))
			}
		}
		if len(failed) > 0 {
			h.log.Error("failed to send update to some chats",
				zap.Strings("failed_chat_ids", failed),
				zap.Int64("id", msg.GetId()),
				zap.String("url", msg.GetUrl()),
				zap.Int32("partition", message.Partition),
				zap.Int64("offset", message.Offset),
			)
		}
		h.log.Info("kafka update processed",
			zap.Int64("id", msg.GetId()),
			zap.String("url", msg.GetUrl()),
			zap.Int("chat_ids_count", len(msg.GetTgChatIds())),
			zap.Int("failed_count", len(failed)),
			zap.Int32("partition", message.Partition),
			zap.Int64("offset", message.Offset),
		)

		// Если приложение упадет ДО этой строки, сообщение будет прочитано снова после рестарта.
		session.MarkMessage(message, "")
	}

	return nil
}

func formatUpdate(req *pbv1.UpdateLinkRequest) string {
	if req.GetDescription() != "" {
		return fmt.Sprintf("🔔 Обновление по ссылке:\n%s\n\n%s", req.GetUrl(), req.GetDescription())
	}
	return fmt.Sprintf("🔔 Обновление по ссылке:\n%s", req.GetUrl())
}
