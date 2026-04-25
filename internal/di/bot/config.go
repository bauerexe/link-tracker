package bot

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/spf13/afero"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
)

var ConfigModule = fx.Options(
	fx.Provide(
		newConfig,
		newSaramaConfig,
	),
)

func newConfig(log *zap.Logger) (config.BotConfig, config.KafkaConfig, error) {
	fs := afero.NewOsFs()

	cfg, err := config.NewBotConfig(fs)
	if err != nil {
		return config.BotConfig{}, config.KafkaConfig{}, fmt.Errorf("load bot config: %w", err)
	}
	cfgKafka, err := config.NewKafkaConfig(fs)
	if err != nil {
		return config.BotConfig{}, config.KafkaConfig{}, fmt.Errorf("load kafka config: %w", err)
	}
	log.Info("init config")
	return cfg, cfgKafka, nil
}

func newSaramaConfig() *sarama.Config {
	return kafka.New()
}
