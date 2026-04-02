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

func newConfig(log *zap.Logger) (*config.ScrapperConfig, error) {
	fs := afero.NewOsFs()

	cfg, err := config.NewScrapperConfig(fs)
	if err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}

	log.Info("init config")
	return &cfg, nil
}
