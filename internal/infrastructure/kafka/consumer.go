package kafka

import (
	"context"
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/zap"
)

type ConsumerBotFromScrapper struct {
	consumer  sarama.ConsumerGroup
	CfgKafka  config.KafkaConfig
	CfgSarama *sarama.Config
	Messages  chan *sarama.ProducerMessage
	log       *zap.Logger
	bot       botapp.BotGateway
}

func NewConsumer(cfgKafka config.KafkaConfig, cfgSarama *sarama.Config, log *zap.Logger, bot botapp.BotGateway) (*ConsumerBotFromScrapper, error) {
	consumer, err := sarama.NewConsumerGroup(cfgKafka.KafkaBrokers, cfgKafka.KafkaConsumerGroup, cfgSarama)
	if log != nil {
		log = log.Named("consumer kafka")
	}
	if err != nil {
		return nil, fmt.Errorf("error creating consumer group: %w", err)
	}
	return &ConsumerBotFromScrapper{
		consumer:  consumer,
		CfgKafka:  cfgKafka,
		CfgSarama: cfgSarama,
		Messages:  nil,
		log:       log,
		bot:       bot,
	}, nil
}

func (c *ConsumerBotFromScrapper) Run(ctx context.Context) error {
	handler := NewHandler(c.log, c.bot)

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
	return c.consumer.Close()
}

func (c *ConsumerBotFromScrapper) consume(ctx context.Context, wg *sync.WaitGroup, consumer sarama.ConsumerGroup) {
	defer wg.Done()

	handler := NewHandler(c.log, c.bot)

	for {
		if err := consumer.Consume(ctx, []string{c.CfgKafka.KafkaTopic}, handler); err != nil {
			c.log.Error("Error on consumer message", zap.Error(err))

			return
		}

		if ctx.Err() != nil {
			return
		}
	}
}
