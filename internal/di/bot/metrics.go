package bot

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const defaultBotMetricsAddr = "0.0.0.0:8011"

var MetricsModule = fx.Options(
	fx.Invoke(runMetricsServer),
)

func runMetricsServer(lc fx.Lifecycle, cfg config.BotConfig, log *zap.Logger) {
	addr := strings.TrimSpace(cfg.BotMetricsAddr)
	if addr == "" {
		addr = defaultBotMetricsAddr
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("bot metrics server listening", zap.String("addr", addr))
			go func() {
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Error("bot metrics server stopped with error", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("stopping bot metrics server")
			return server.Shutdown(ctx)
		},
	})
}
