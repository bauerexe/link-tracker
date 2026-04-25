package scrapper

import (
	"fmt"

	"github.com/spf13/afero"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
)

var ConfigModule = fx.Options(
	fx.Provide(
		newConfig,
	),
)

func newConfig(log *zap.Logger) (*config.ScrapperConfig, config.KafkaConfig, error) {
	fs := afero.NewOsFs()

	cfg, err := config.NewScrapperConfig(fs)
	if err != nil {
		return nil, config.KafkaConfig{}, fmt.Errorf("init config: %w", err)
	}
	cfgKafka, err := config.NewKafkaConfig(fs)
	if err != nil {
		return nil, config.KafkaConfig{}, fmt.Errorf("load kafka config: %w", err)
	}
	log.Info("init config")
	return &cfg, cfgKafka, nil
}
