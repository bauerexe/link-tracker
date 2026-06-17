package scrapper

import (
	"context"
	"math"
	"net"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	stackoverflow "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/gateway/scrapper/clients"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/observability/instrumentation"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	httpTimeout             = 10 * time.Second
	httpKeepAlive           = 30 * time.Second
	httpMaxIdleConns        = 100
	httpIdleConnTimeout     = 90 * time.Second
	httpTLSHandshakeTimeout = 10 * time.Second
)

var HTTPModule = fx.Options(
	fx.Provide(
		newHTTPClient,
		newResilienceConfig,
		newGitHubClient,
		newStackOverflowClient,
	),
)

func newResilienceConfig(cfg *config.ScrapperConfig) clients.ResilienceConfig {
	retryable := make(map[int]struct{}, len(cfg.RetryableStatuses))
	for _, code := range cfg.RetryableStatuses {
		retryable[code] = struct{}{}
	}
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "external-http",
		MaxRequests: cfg.CBMaxRequests,
		Interval:    cfg.CBInterval,
		Timeout:     cfg.CBTimeout,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < cfg.CBMinRequests || c.Requests == 0 {
				return false
			}
			const i = float64(100)
			failureRate := math.Round((float64(c.TotalFailures) / float64(c.Requests)) * i)
			return failureRate >= cfg.CBFailureRate
		},
	})
	return clients.ResilienceConfig{RetryMaxAttempts: cfg.RetryMaxAttempts, RetryDelay: cfg.RetryDelay, RetryableStatuses: retryable, CircuitBreaker: cb}
}

func newHTTPClient(lc fx.Lifecycle, log *zap.Logger, cfg *config.ScrapperConfig) *http.Client {
	timeout := httpTimeout
	if cfg.HTTPTimeout > 0 {
		timeout = cfg.HTTPTimeout
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: httpKeepAlive,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          httpMaxIdleConns,
		IdleConnTimeout:       httpIdleConnTimeout,
		TLSHandshakeTimeout:   httpTLSHandshakeTimeout,
		ExpectContinueTimeout: time.Second,
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			log.Info("closing idle http connections")
			transport.CloseIdleConnections()
			return nil
		},
	})

	return client
}

func newGitHubClient(httpClient *http.Client, cfg *config.ScrapperConfig, log *zap.Logger, resilience clients.ResilienceConfig) github.Client {
	return instrumentation.NewMeasuredGitHubClient(clients.NewGitHubClient(httpClient, cfg.GitHubToken, log, resilience))
}

func newStackOverflowClient(httpClient *http.Client, cfg *config.ScrapperConfig, log *zap.Logger, resilience clients.ResilienceConfig) stackoverflow.Client {
	return instrumentation.NewMeasuredStackOverflowClient(clients.NewStackOverflowClient(httpClient, cfg.StackExchangeKey, log, resilience))
}
