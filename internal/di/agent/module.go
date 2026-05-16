package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/IBM/sarama"
	"github.com/spf13/afero"
	appagent "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/agent"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	kafkainfra "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Module = fx.Options(
	fx.Provide(
		newLogger,
		newConfig,
		newSaramaConfig,
		newSummarizer,
		newProcessor,
		newConsumer,
	),
	fx.Invoke(runConsumer),
)

func newLogger() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	log, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}

	log = log.Named("agent").With(zap.String("service", "agent"))
	return log, nil
}

func newConfig(log *zap.Logger) (config.AgentConfig, config.KafkaConfig, error) {
	fs := afero.NewOsFs()

	agentCfg, err := config.NewAgentConfig(fs)
	if err != nil {
		return config.AgentConfig{}, config.KafkaConfig{}, fmt.Errorf("load ai-agent config: %w", err)
	}

	kafkaCfg, err := config.NewKafkaConfig(fs)
	if err != nil {
		return config.AgentConfig{}, config.KafkaConfig{}, fmt.Errorf("load kafka config: %w", err)
	}

	log.Info("init config")
	return agentCfg, kafkaCfg, nil
}

func newSaramaConfig() *sarama.Config {
	return kafkainfra.New()
}

func newSummarizer(cfg config.AgentConfig, log *zap.Logger) (appagent.Summarizer, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Summarization.Provider))
	if provider == "" {
		provider = config.AgentSummarizationProviderStub
	}

	switch provider {
	case config.AgentSummarizationProviderStub:
		log.Info("init summarizer", zap.String("provider", provider))
		return appagent.NewStubSummarizer(), nil

	case config.AgentSummarizationProviderGemini:
		summarizer, err := appagent.NewGeminiSummarizer(
			cfg.Summarization.GeminiAPIKey,
			cfg.Summarization.GeminiModel,
			cfg.Summarization.RequestTimeout,
		)
		if err != nil {
			return nil, fmt.Errorf("init gemini summarizer: %w", err)
		}

		log.Info("init summarizer",
			zap.String("provider", provider),
			zap.String("model", cfg.Summarization.GeminiModel),
		)

		return summarizer, nil

	default:
		return nil, fmt.Errorf("unknown summarization provider %q", provider)
	}
}

func newProcessor(cfg config.AgentConfig, summarizer appagent.Summarizer) *appagent.Processor {
	return appagent.NewProcessor(cfg, summarizer)
}

func newConsumer(
	cfgKafka config.KafkaConfig,
	cfgAgent config.AgentConfig,
	cfgSarama *sarama.Config,
	log *zap.Logger,
	processor *appagent.Processor,
) (*kafkainfra.AgentConsumer, error) {
	if !cfgKafka.KafkaEnabled {
		log.Info("kafka not enabled")
		return kafkainfra.NewNoopAgentConsumer(log), nil
	}

	consumer, err := kafkainfra.NewAgentConsumer(cfgKafka, cfgAgent, cfgSarama, log, processor)
	if err != nil {
		return nil, fmt.Errorf("new ai-agent consumer failed: %w", err)
	}

	return consumer, nil
}

func runConsumer(
	lc fx.Lifecycle,
	cfg config.KafkaConfig,
	consumer *kafkainfra.AgentConsumer,
	log *zap.Logger,
) {
	if !cfg.KafkaEnabled {
		log.Info("kafka consumer disabled")
		return
	}

	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("starting ai-agent consumer")

			runCtx, c := context.WithCancel(context.Background())
			cancel = c

			go func() {
				if err := consumer.Run(runCtx); err != nil {
					log.Error("ai-agent consumer stopped", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(_ context.Context) error {
			if cancel != nil {
				cancel()
			}

			return consumer.Close()
		},
	})
}
