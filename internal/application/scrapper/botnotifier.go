package scrapperapp

import (
	"context"
	"fmt"

	"github.com/riferrei/srclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	dbtx "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/db"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka/outbox"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
	"go.uber.org/zap"
)

type GRPCBotNotifier struct {
	client pbv1.BotClient
	log    *zap.Logger
}

func NewGRPCBotNotifier(client pbv1.BotClient, log *zap.Logger) *GRPCBotNotifier {
	if log != nil {
		log = log.Named("bot_notifier")
	}
	return &GRPCBotNotifier{client: client, log: log}
}

func (n *GRPCBotNotifier) Notify(ctx context.Context, url, description string, chatIDs []int64) error {
	_, err := n.client.UpdateLink(ctx, &pbv1.UpdateLinkRequest{
		Url:         url,
		Description: description,
		TgChatIds:   chatIDs,
	})
	if err != nil && n.log != nil {
		if n.log != nil {
			n.log.Error("UpdateLink failed", zap.Error(err))
		}
		return fmt.Errorf("notify err: %w", err)
	}
	return nil
}

type KafkaBotNotifier struct {
	cfgKafka config.KafkaConfig
	log      *zap.Logger
	avro     *kafka.AvroCodec
}

func NewKafkaBotNotifier(cfgKafka config.KafkaConfig, log *zap.Logger) (*KafkaBotNotifier, error) {
	if log != nil {
		log = log.Named("bot_notifier")
	}

	client := srclient.NewSchemaRegistryClient(cfgKafka.SchemaRegistryURL)

	schema, err := client.GetLatestSchema(cfgKafka.SchemaSubject)
	if err != nil {
		return nil, fmt.Errorf("get latest avro schema: %w", err)
	}

	codec, err := kafka.NewAvroCodec(schema.Schema())
	if err != nil {
		return nil, fmt.Errorf("create avro codec: %w", err)
	}

	return &KafkaBotNotifier{
		cfgKafka: cfgKafka,
		log:      log,
		avro:     codec,
	}, nil
}

func (n *KafkaBotNotifier) Notify(ctx context.Context, url, description string, chatIDs []int64) error {
	tx, ok := dbtx.TxFromContext(ctx)
	if !ok {
		return fmt.Errorf("outbox tx not found in context")
	}

	msg := kafka.UpdateLinkAvro{
		URL:         url,
		Description: description,
		TgChatIDs:   chatIDs,
	}

	value, err := n.avro.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal avro updateLinkRequest failed: %w", err)
	}

	err = outbox.InsertTx(ctx, tx, outbox.Message{
		Topic:      n.cfgKafka.KafkaTopic,
		MessageKey: []byte(url),
		Payload:    value,
	})
	if err != nil {
		return fmt.Errorf("insert message to outbox: %w", err)
	}

	return nil
}
