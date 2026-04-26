package kafka

import (
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/zap"
)

type ProducerScrapperToBot struct {
	producer  sarama.AsyncProducer
	log       *zap.Logger
	ErrorChan <-chan *sarama.ProducerError
	CfgKafka  config.KafkaConfig
	CfgSarama *sarama.Config
	Messages  chan *sarama.ProducerMessage
}

func NewProducer(cfgKafka config.KafkaConfig, cfgSarama *sarama.Config, log *zap.Logger) (*ProducerScrapperToBot, error) {
	cfgSarama.Producer.Return.Successes = true
	cfgSarama.Producer.Return.Errors = true
	producer, err := sarama.NewAsyncProducer(cfgKafka.KafkaBrokers, cfgSarama)
	if log != nil {
		log = log.Named("producer kafka")
	}
	if err != nil {
		return nil, fmt.Errorf("new async producer fail: %w", err)
	}

	return &ProducerScrapperToBot{
		producer:  producer,
		log:       log,
		CfgKafka:  cfgKafka,
		CfgSarama: cfgSarama,
		Messages:  make(chan *sarama.ProducerMessage),
		ErrorChan: producer.Errors(),
	}, nil
}

func NewNoopProducer(log *zap.Logger) *ProducerScrapperToBot {
	return &ProducerScrapperToBot{
		log: log,
	}
}

func (p *ProducerScrapperToBot) Run() error {
	if p == nil || p.producer == nil {
		if p != nil && p.log != nil {
			p.log.Info("kafka producer disabled; skip run")
		}
		return nil
	}

	var wg sync.WaitGroup

	const delta = 2
	wg.Add(delta)
	go func(producer sarama.AsyncProducer) {
		defer wg.Done()
		for success := range producer.Successes() {
			p.log.Info("producer success",
				zap.String("topic", success.Topic),
				zap.String("partition", string(success.Partition)),
			)
		}
	}(p.producer)

	go func(producer sarama.AsyncProducer) {
		defer wg.Done()
		for err := range producer.Errors() {
			p.log.Error("producer error", zap.Error(err))
		}
	}(p.producer)

	for msg := range p.Messages {
		p.producer.Input() <- msg
	}
	p.producer.AsyncClose()

	wg.Wait()

	return nil
}
