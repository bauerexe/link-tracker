package config

import (
	"os"
	"strings"

	"github.com/byrnedo/typesafe-config/parse"
	"github.com/spf13/afero"
)

type KafkaConfig struct {
	KafkaTopic         string   `config:"kafka_topic"`
	KafkaConsumerGroup string   `config:"kafka_consumer_group"`
	KafkaBrokers       []string `config:"kafka_brokers"`
	KafkaEnabled       bool     `config:"kafka_enabled"`
}

func NewKafkaConfig(fs afero.Fs) (KafkaConfig, error) {
	file, err := afero.ReadFile(fs, EnvFile)
	if err != nil {
		return KafkaConfig{}, ErrReadFile
	}

	tree, err := parse.ParseBytes(file)
	if err != nil {
		return KafkaConfig{}, ErrParseFile
	}

	config := &KafkaConfig{}
	parse.Populate(config, tree.GetConfig(), "kafka")

	if brokersEnv := strings.TrimSpace(os.Getenv("KAFKA_BROKERS")); brokersEnv != "" {
		config.KafkaBrokers = splitKafkaBrokers(brokersEnv)
	}

	return *config, nil
}

func splitKafkaBrokers(raw string) []string {
	parts := strings.Split(raw, ",")

	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		broker := strings.TrimSpace(part)
		broker = strings.Trim(broker, `"[]`)

		if broker != "" {
			brokers = append(brokers, broker)
		}
	}

	return brokers
}
