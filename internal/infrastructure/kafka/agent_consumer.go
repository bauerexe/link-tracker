package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/IBM/sarama"
	"github.com/riferrei/srclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/zap"
)

type AgentConsumer struct {
	consumer   sarama.ConsumerGroup
	producer   messageProducer
	inputTopic string
	handler    *AgentHandler
	log        *zap.Logger
}

func NewAgentConsumer(
	cfgKafka config.KafkaConfig,
	cfgAgent config.AgentConfig,
	cfgSarama *sarama.Config,
	log *zap.Logger,
	processor UpdateProcessor,
) (*AgentConsumer, error) {
	if processor == nil {
		return nil, errors.New("agent processor is nil")
	}
	if strings.TrimSpace(cfgAgent.OutputTopic) == "" {
		return nil, errors.New("ai-agent output topic is empty")
	}
	if log == nil {
		log = zap.NewNop()
	} else {
		log = log.Named("agent consumer")
	}

	consumer, err := sarama.NewConsumerGroup(cfgKafka.KafkaBrokers, cfgKafka.KafkaConsumerGroup, cfgSarama)
	if err != nil {
		return nil, fmt.Errorf("create consumer group: %w", err)
	}

	producer, err := sarama.NewSyncProducer(cfgKafka.KafkaBrokers, cfgSarama)
	if err != nil {
		_ = consumer.Close()
		return nil, fmt.Errorf("create sync producer: %w", err)
	}

	schemaRegistryClient := srclient.NewSchemaRegistryClient(cfgKafka.SchemaRegistryURL)
	schema, err := schemaRegistryClient.GetLatestSchema(cfgKafka.SchemaSubject)
	if err != nil {
		_ = producer.Close()
		_ = consumer.Close()
		return nil, fmt.Errorf("get avro schema from registry: %w", err)
	}

	codec, err := NewAvroCodec(schema.Schema())
	if err != nil {
		_ = producer.Close()
		_ = consumer.Close()
		return nil, fmt.Errorf("create avro codec: %w", err)
	}

	return &AgentConsumer{
		consumer:   consumer,
		producer:   producer,
		inputTopic: cfgKafka.KafkaTopic,
		handler:    NewAgentHandler(log, processor, producer, cfgAgent.OutputTopic, cfgKafka.DLQTopic, codec),
		log:        log,
	}, nil
}

func NewNoopAgentConsumer(log *zap.Logger) *AgentConsumer {
	if log == nil {
		log = zap.NewNop()
	}

	return &AgentConsumer{log: log}
}

func (c *AgentConsumer) Run(ctx context.Context) error {
	if c.consumer == nil {
		c.log.Info("kafka consumer disabled; skip run")
		return nil
	}

	for {
		if err := c.consumer.Consume(ctx, []string{c.inputTopic}, c.handler); err != nil {
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

func (c *AgentConsumer) Close() error {
	var closeErr error

	if c.consumer != nil {
		if err := c.consumer.Close(); err != nil {
			closeErr = fmt.Errorf("close consumer: %w", err)
		}
	}

	if c.producer != nil {
		if err := c.producer.Close(); err != nil {
			if closeErr != nil {
				return fmt.Errorf("%v; close producer: %w", closeErr, err)
			}
			return fmt.Errorf("close producer: %w", err)
		}
	}

	return closeErr
}
