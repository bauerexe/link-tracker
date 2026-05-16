package kafka

import (
	"context"

	"github.com/IBM/sarama"
	agentapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/agent"
	"go.uber.org/zap"
)

type messageProducer interface {
	SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error)
	Close() error
}

type UpdateProcessor interface {
	Process(ctx context.Context, update agentapp.Update) (agentapp.Update, bool, error)
}

type AgentHandler struct {
	log         *zap.Logger
	processor   UpdateProcessor
	producer    messageProducer
	outputTopic string
	dlqTopic    string
	avro        *AvroCodec
}

func NewAgentHandler(
	log *zap.Logger,
	processor UpdateProcessor,
	producer messageProducer,
	outputTopic string,
	dlqTopic string,
	avro *AvroCodec,
) *AgentHandler {
	if log == nil {
		log = zap.NewNop()
	} else {
		log = log.Named("agent consumer handler")
	}

	return &AgentHandler{
		log:         log,
		processor:   processor,
		producer:    producer,
		outputTopic: outputTopic,
		dlqTopic:    dlqTopic,
		avro:        avro,
	}
}

func (h *AgentHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *AgentHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *AgentHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var raw UpdateLinkAvro

		if err := h.avro.Unmarshal(message.Value, &raw); err != nil {
			h.log.Error("failed to unmarshal avro", zap.Error(err))
			h.sendToDLQ(message, "failed to unmarshal avro: "+err.Error())
			session.MarkMessage(message, "")
			continue
		}

		processed, keep, err := h.processor.Process(session.Context(), toAgentUpdate(raw))
		if err != nil {
			h.log.Error("failed to process update", zap.Error(err), zap.Int64("id", raw.ID), zap.String("url", raw.URL))
			h.sendToDLQ(message, "failed to process update: "+err.Error())
			session.MarkMessage(message, "")
			continue
		}

		if !keep {
			h.log.Info("update filtered", zap.Int64("id", raw.ID), zap.String("url", raw.URL))
			session.MarkMessage(message, "")
			continue
		}

		payload, err := h.avro.Marshal(fromAgentUpdate(processed))
		if err != nil {
			h.log.Error("failed to marshal avro", zap.Error(err), zap.Int64("id", raw.ID), zap.String("url", raw.URL))
			h.sendToDLQ(message, "failed to marshal avro: "+err.Error())
			session.MarkMessage(message, "")
			continue
		}

		if _, _, err = h.producer.SendMessage(&sarama.ProducerMessage{
			Topic: h.outputTopic,
			Key:   producerKey(message, raw.URL),
			Value: sarama.ByteEncoder(payload),
		}); err != nil {
			h.log.Error("failed to publish processed update", zap.Error(err), zap.Int64("id", raw.ID), zap.String("url", raw.URL))
			h.sendToDLQ(message, "failed to publish processed update: "+err.Error())
			session.MarkMessage(message, "")
			continue
		}

		h.log.Info("update processed", zap.Int64("id", raw.ID), zap.String("url", raw.URL), zap.String("output_topic", h.outputTopic))
		session.MarkMessage(message, "")
	}

	return nil
}

func (h *AgentHandler) sendToDLQ(message *sarama.ConsumerMessage, reason string) {
	if h.producer == nil || h.dlqTopic == "" {
		h.log.Error("dlq producer is not configured", zap.String("reason", reason))
		return
	}

	_, _, err := h.producer.SendMessage(&sarama.ProducerMessage{
		Topic: h.dlqTopic,
		Key:   sarama.ByteEncoder(message.Key),
		Value: sarama.ByteEncoder(message.Value),
		Headers: []sarama.RecordHeader{{
			Key:   []byte("x-dlq-reason"),
			Value: []byte(reason),
		}},
	})
	if err != nil {
		h.log.Error("failed to send message to dlq", zap.Error(err), zap.String("dlq_topic", h.dlqTopic), zap.String("reason", reason))
		return
	}

	h.log.Info("message sent to dlq", zap.String("dlq_topic", h.dlqTopic), zap.String("reason", reason))
}

func producerKey(message *sarama.ConsumerMessage, fallbackKey string) sarama.Encoder {
	if len(message.Key) != 0 {
		return sarama.ByteEncoder(message.Key)
	}

	return sarama.StringEncoder(fallbackKey)
}

func toAgentUpdate(update UpdateLinkAvro) agentapp.Update {
	return agentapp.Update{
		ID:          update.ID,
		URL:         update.URL,
		Description: update.Description,
		TgChatIDs:   update.TgChatIDs,
	}
}

func fromAgentUpdate(update agentapp.Update) UpdateLinkAvro {
	return UpdateLinkAvro{
		ID:          update.ID,
		URL:         update.URL,
		Description: update.Description,
		TgChatIDs:   update.TgChatIDs,
	}
}
