package scrapper

import (
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var LoggerModule = fx.Options(
	fx.Provide(
		newLogger,
	),
)

func newLogger() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	log, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build zap logger: %w", err)
	}

	log = log.Named("scrapper").With(zap.String("service", "scrapper"))
	return log, nil
}
