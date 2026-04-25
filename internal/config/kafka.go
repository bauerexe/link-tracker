package config

import (
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
	var tree *parse.Tree
	var err error
	file, err := afero.ReadFile(fs, envFile)
	if err != nil {
		return KafkaConfig{}, ErrReadFile
	}
	if tree, err = parse.ParseBytes(file); err != nil {
		return KafkaConfig{}, ErrParseFile
	}
	config := &KafkaConfig{}
	parse.Populate(config, tree.GetConfig(), "kafka")
	return *config, nil
}
