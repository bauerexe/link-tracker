package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/afero"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/db"
	ormrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/scrapper/postgres"
	rawrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/scrapper/rawpostgres"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/gateway/scrapper/checkers"
	controller "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

const (
	httpTimeout             = 10 * time.Second
	httpKeepAlive           = 30 * time.Second
	httpMaxIdleConns        = 100
	httpIdleConnTimeout     = 90 * time.Second
	httpTLSHandshakeTimeout = 10 * time.Second
)

func main() {
	fx.New(
		fx.Provide(
			newConfig,
			newZap,
			newPostgresPool,
			newChatRepo,
			newLinkRepo,
			newScrapperServer,
			newScrapperApp,
			newAppContext,
			newHTTPClient,
			newBotConn,
			newBotClient,
			newBotNotifier,
			newCheckers,
			newScheduler,
		),
		fx.Invoke(runScrapper, runScheduler),
	).Run()
}

func newAppContext(lc fx.Lifecycle, log *zap.Logger) context.Context {
	appCtx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			log.Info("stopping app context")
			cancel()
			return nil
		},
	})
	return appCtx
}

func newConfig(log *zap.Logger) (*config.ScrapperConfig, error) {
	fs := afero.NewOsFs()
	cfg, err := config.NewScrapperConfig(fs)
	if err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}
	log.Info("init config")
	return &cfg, nil
}

func newZap() (*zap.Logger, error) {
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

func newPostgresPool(lc fx.Lifecycle, cfg *config.ScrapperConfig, log *zap.Logger) (*pgxpool.Pool, error) {
	pool, err := db.NewPostgresPool(context.Background(), cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			log.Info("closing postgres pool")
			pool.Close()
			return nil
		},
	})

	return pool, nil
}

func newChatRepo(cfg *config.ScrapperConfig, pool *pgxpool.Pool) (scrapperapp.ChatRepository, error) {
	switch cfg.DBAccessType {
	case "sql":
		return rawrepo.NewChatRepository(pool), nil
	case "orm":
		return ormrepo.NewChatRepository(pool), nil
	default:
		return nil, config.ErrParseFile
	}
}

func newLinkRepo(cfg *config.ScrapperConfig, pool *pgxpool.Pool) (scrapperapp.LinkRepository, error) {
	switch cfg.DBAccessType {
	case "sql":
		return rawrepo.NewLinkRepository(pool), nil
	case "orm":
		return ormrepo.NewLinkRepository(pool), nil
	default:
		return nil, config.ErrParseFile
	}
}

func newScrapperServer(log *zap.Logger, chatRepo scrapperapp.ChatRepository, linkRepo scrapperapp.LinkRepository) pbv1.ScrapperServer {
	return controller.New(log, chatRepo, linkRepo)
}

func newScrapperApp(server pbv1.ScrapperServer, log *zap.Logger, cfg *config.ScrapperConfig) *scrapperapp.Scrapper {
	sa := scrapperapp.New(server, log, cfg)
	return &sa
}

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

func newCheckers(httpClient *http.Client, cfg *config.ScrapperConfig, log *zap.Logger) []scrapperapp.Checker {
	return []scrapperapp.Checker{
		checkers.NewGitHubChecker(httpClient, cfg.GitHubToken, log),
		checkers.NewStackOverflowChecker(httpClient, cfg.StackExchangeKey, log),
	}
}

func newBotConn(lc fx.Lifecycle, cfg *config.ScrapperConfig, log *zap.Logger) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(cfg.BotAddrGRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create grpc client: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			log.Info("closing bot grpc conn")
			return conn.Close()
		},
	})

	return conn, nil
}

func newBotClient(conn *grpc.ClientConn) pbv1.BotClient {
	return pbv1.NewBotClient(conn)
}

func newBotNotifier(client pbv1.BotClient, log *zap.Logger) scrapperapp.BotNotifier {
	return scrapperapp.NewGRPCBotNotifier(client, log)
}

func newScheduler(
	linkRepo scrapperapp.LinkRepository,
	notifier scrapperapp.BotNotifier,
	checkers []scrapperapp.Checker,
	log *zap.Logger,
	cfg *config.ScrapperConfig,
) (*scrapperapp.Scheduler, error) {
	interval := time.Duration(cfg.MinutesIntervalCheck) * time.Minute

	s, err := scrapperapp.NewScheduler(&scrapperapp.Scheduler{
		Links:    linkRepo,
		Notifier: notifier,
		Checkers: checkers,
		Log:      log,
		Interval: interval,
	})

	if err != nil {
		return nil, fmt.Errorf("new scheduler: %w", err)
	}
	return s, nil
}

func runScrapper(appCtx context.Context, lc fx.Lifecycle, scrapper *scrapperapp.Scrapper, log *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("start scrapper")
			go func() {
				scrapper.Run(appCtx)
			}()
			return nil
		},
	})
}

func runScheduler(appCtx context.Context, lc fx.Lifecycle, scheduler *scrapperapp.Scheduler, log *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("start scheduler")
			go scheduler.Run(appCtx)
			return nil
		},
	})
}
