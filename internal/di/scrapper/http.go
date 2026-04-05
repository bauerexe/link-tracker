package scrapper

import (
	"context"
	"net"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	stackoverflow "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/gateway/scrapper/clients"
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
		newGitHubClient,
		newStackOverflowClient,
	),
)

func newHTTPClient(lc fx.Lifecycle, log *zap.Logger) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   httpTimeout,
			KeepAlive: httpKeepAlive,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          httpMaxIdleConns,
		IdleConnTimeout:       httpIdleConnTimeout,
		TLSHandshakeTimeout:   httpTLSHandshakeTimeout,
		ExpectContinueTimeout: time.Second,
	}

	client := &http.Client{
		Timeout:   httpTimeout,
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

func newGitHubClient(httpClient *http.Client, cfg *config.ScrapperConfig, log *zap.Logger) github.Client {
	return clients.NewGitHubClient(httpClient, cfg.GitHubToken, log)
}

func newStackOverflowClient(httpClient *http.Client, cfg *config.ScrapperConfig, log *zap.Logger) stackoverflow.Client {
	return clients.NewStackOverflowClient(httpClient, cfg.StackExchangeKey, log)
}
