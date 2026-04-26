package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/riferrei/srclient"
	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/zap"
)

type ConsumerBotFromScrapper struct {
	consumer    sarama.ConsumerGroup
	CfgKafka    config.KafkaConfig
	CfgSarama   *sarama.Config
	Messages    chan *sarama.ProducerMessage
	log         *zap.Logger
	bot         botapp.BotGateway
	dlqProducer sarama.SyncProducer
	codec       *AvroCodec
}

func NewConsumer(cfgKafka config.KafkaConfig, cfgSarama *sarama.Config, log *zap.Logger, bot botapp.BotGateway) (*ConsumerBotFromScrapper, error) {
	consumer, err := sarama.NewConsumerGroup(cfgKafka.KafkaBrokers, cfgKafka.KafkaConsumerGroup, cfgSarama)
	if log != nil {
		log = log.Named("consumer kafka")
	}
	if err != nil {
		return nil, fmt.Errorf("error creating consumer group: %w", err)
	}
	dlqProducer, err := sarama.NewSyncProducer(cfgKafka.KafkaBrokers, cfgSarama)
	if err != nil {
		_ = consumer.Close()
		return nil, fmt.Errorf("error creating dlq producer: %w", err)
	}
	schemaRegistryClient := srclient.NewSchemaRegistryClient(cfgKafka.SchemaRegistryURL)
	schema, err := schemaRegistryClient.GetLatestSchema(cfgKafka.SchemaSubject)
	if err != nil {
		return nil, fmt.Errorf("get avro schema from registry: %w", err)
	}
	codec, err := NewAvroCodec(schema.Schema())
	if err != nil {
		_ = consumer.Close()
		return nil, fmt.Errorf("error creating avro codec: %w", err)
	}
	return &ConsumerBotFromScrapper{
		consumer:    consumer,
		CfgKafka:    cfgKafka,
		CfgSarama:   cfgSarama,
		Messages:    nil,
		log:         log,
		bot:         bot,
		dlqProducer: dlqProducer,
		codec:       codec,
	}, nil
}

func NewNoopConsumer(log *zap.Logger) *ConsumerBotFromScrapper {
	return &ConsumerBotFromScrapper{log: log}
}

func (c *ConsumerBotFromScrapper) Run(ctx context.Context) error {
	if c.consumer == nil {
		c.log.Info("kafka consumer disabled; skip run")
		return nil
	}
	handler := NewHandler(
		c.log,
		c.bot,
		c.dlqProducer,
		c.CfgKafka.DLQTopic,
		c.CfgKafka.MaxRetries,
		c.codec,
	)

	for {
		if err := c.consumer.Consume(ctx, []string{c.CfgKafka.KafkaTopic}, handler); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("consume kafka: %w", err)
		}

		if ctx.Err() != nil {
			return nil
		}
	}
}

func (c *ConsumerBotFromScrapper) Close() error {
	return fmt.Errorf("close called: %w", c.consumer.Close())
}
