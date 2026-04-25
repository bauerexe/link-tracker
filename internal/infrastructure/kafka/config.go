package kafka

import (
	"time"

	"github.com/IBM/sarama"
)

func New(opts ...func(*sarama.Config)) *sarama.Config {
	cfg := sarama.NewConfig()

	cfg.Version = sarama.V3_9_0_0

	cfg.Producer.Partitioner = sarama.NewHashPartitioner
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Compression = sarama.CompressionGZIP

	//cfg.Consumer.Offsets.Initial = sarama.OffsetNewest не понял зачем это, если упадет сервис, то он не обработает старые сообщения
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Consumer.Offsets.AutoCommit.Enable = false
	cfg.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second

	for _, o := range opts {
		o(cfg)
	}

	return cfg
}
