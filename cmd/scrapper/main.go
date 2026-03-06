package main

import (
	"context"
	"net/http"
	"time"

	"github.com/spf13/afero"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/gateway/scrapper/checkers"
	controller "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/scrapper"
	repository "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/scrapper/inmemory"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

func main() {
	fx.New(
		fx.Provide(
			newConfig,
			newZap,
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
		return nil, err
	}
	log.Info("init config")
	return &cfg, nil
}

func newZap() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	// cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	log, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	log = log.Named("scrapper").With(zap.String("service", "scrapper"))
	return log, nil
}

func newChatRepo() scrapperapp.ChatRepository {
	return repository.NewChatRepository()
}

func newLinkRepo() scrapperapp.LinkRepository {
	return repository.NewLinkRepository()
}

func newScrapperServer(log *zap.Logger, chatRepo scrapperapp.ChatRepository, linkRepo scrapperapp.LinkRepository) pbv1.ScrapperServer {
	return controller.New(log, chatRepo, linkRepo)
}

func newScrapperApp(server pbv1.ScrapperServer, log *zap.Logger, cfg *config.ScrapperConfig) scrapperapp.Scrapper {
	return scrapperapp.New(server, log, cfg)
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
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
		return nil, err
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
	interval := time.Duration(cfg.MinutesIntervalCheck * int(time.Minute))

	return scrapperapp.NewScheduler(&scrapperapp.Scheduler{
		Links:    linkRepo,
		Notifier: notifier,
		Checkers: checkers,
		Log:      log,
		Interval: interval,
	})
}

func runScrapper(appCtx context.Context, lc fx.Lifecycle, scrapper scrapperapp.Scrapper, log *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("start scrapper")
			go func() {
				scrapper.Run(appCtx)
			}()
			return nil
		},
		OnStop: nil,
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
