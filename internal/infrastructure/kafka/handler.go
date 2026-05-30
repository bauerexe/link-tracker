package kafka

import (
	"fmt"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/observability"
	"go.uber.org/zap"
)

type Handler struct {
	log        *zap.Logger
	bot        botapp.BotGateway
	dlq        sarama.SyncProducer
	dlqTopic   string
	maxRetries int
	avro       *AvroCodec
}

func NewHandler(
	log *zap.Logger,
	bot botapp.BotGateway,
	dlq sarama.SyncProducer,
	dlqTopic string,
	maxRetries int,
	avro *AvroCodec,
) *Handler {
	if log != nil {
		log = log.Named("kafka consumer handler")
	}

	if maxRetries < 1 {
		maxRetries = 1
	}

	return &Handler{
		log:        log,
		bot:        bot,
		dlq:        dlq,
		dlqTopic:   dlqTopic,
		maxRetries: maxRetries,
		avro:       avro,
	}
}

func (h *Handler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *Handler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *Handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var avroMsg UpdateLinkAvro

		if err := h.avro.Unmarshal(message.Value, &avroMsg); err != nil {
			h.log.Error("failed to unmarshal avro",
				zap.Error(err),
				zap.Int32("partition", message.Partition),
				zap.Int64("offset", message.Offset),
			)

			h.sendToDLQ(message, "failed to unmarshal avro: "+err.Error())
			session.MarkMessage(message, "")
			continue
		}

		if len(avroMsg.TgChatIDs) == 0 {
			h.log.Debug("tgChatIds is empty",
				zap.Int64("id", avroMsg.ID),
				zap.String("url", avroMsg.URL),
			)

			h.sendToDLQ(message, "tgChatIds is empty")
			session.MarkMessage(message, "")
			continue
		}

		if h.processWithRetries(&avroMsg, message) {
			h.log.Info("kafka update processed",
				zap.Int64("id", avroMsg.ID),
				zap.String("url", avroMsg.URL),
				zap.Int("chat_ids_count", len(avroMsg.TgChatIDs)),
				zap.Int("failed_count", 0),
				zap.Int32("partition", message.Partition),
				zap.Int64("offset", message.Offset),
			)

			session.MarkMessage(message, "")
			continue
		}

		h.sendToDLQ(message, "max retries exceeded")
		session.MarkMessage(message, "")
	}

	return nil
}

func (h *Handler) processWithRetries(msg *UpdateLinkAvro, message *sarama.ConsumerMessage) bool {
	for attempt := 1; attempt <= h.maxRetries; attempt++ {
		failed := h.processMessage(msg)

		if len(failed) == 0 {
			return true
		}

		h.log.Error("failed to send update to some chats",
			zap.Strings("failed_chat_ids", failed),
			zap.Int64("id", msg.ID),
			zap.String("url", msg.URL),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", h.maxRetries),
			zap.Int32("partition", message.Partition),
			zap.Int64("offset", message.Offset),
		)

		if attempt < h.maxRetries {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return false
}

func (h *Handler) processMessage(msg *UpdateLinkAvro) []string {
	text := formatUpdate(msg)

	var failed []string

	for _, chatID := range msg.TgChatIDs {
		if err := h.bot.SendMessage(chatID, 0, text); err != nil {
			h.log.Error("failed to send telegram message",
				zap.Error(err),
				zap.Int64("chat_id", chatID),
				zap.Int64("id", msg.ID),
				zap.String("url", msg.URL),
			)

			failed = append(failed, strconv.FormatInt(chatID, 10))
			continue
		}
		observability.IncSentNotification()
	}

	return failed
}

func (h *Handler) sendToDLQ(message *sarama.ConsumerMessage, reason string) {
	if h.dlq == nil || h.dlqTopic == "" {
		h.log.Error("dlq producer is not configured",
			zap.String("reason", reason),
			zap.Int32("partition", message.Partition),
			zap.Int64("offset", message.Offset),
		)
		return
	}

	_, _, err := h.dlq.SendMessage(&sarama.ProducerMessage{
		Topic: h.dlqTopic,
		Key:   sarama.ByteEncoder(message.Key),
		Value: sarama.ByteEncoder(message.Value),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("x-dlq-reason"),
				Value: []byte(reason),
			},
		},
	})

	if err != nil {
		h.log.Error("failed to send message to dlq",
			zap.Error(err),
			zap.String("dlq_topic", h.dlqTopic),
			zap.String("reason", reason),
		)
		return
	}

	h.log.Info("message sent to dlq",
		zap.String("dlq_topic", h.dlqTopic),
		zap.String("reason", reason),
	)
}

func formatUpdate(req *UpdateLinkAvro) string {
	if req.Description != "" {
		return fmt.Sprintf("🔔 Обновление по ссылке:\n%s\n\n%s", req.URL, req.Description)
	}
	return fmt.Sprintf("🔔 Обновление по ссылке:\n%s", req.URL)
}
