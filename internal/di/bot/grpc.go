package bot

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/observability/instrumentation"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

var GRPCModule = fx.Options(
	fx.Provide(
		newScrapperConn,
		newScrapperClient,
	),
)

func newScrapperConn(
	lc fx.Lifecycle,
	cfg config.BotConfig,
	log *zap.Logger,
) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		cfg.ScrapperAddrGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create scrapper grpc client: %w", err)
	}

	log.Info("connected to scrapper", zap.String("addr", cfg.ScrapperAddrGRPC))

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			log.Info("closing scrapper connection")
			return conn.Close()
		},
	})

	return conn, nil
}

func newScrapperClient(conn *grpc.ClientConn) pbv1.ScrapperClient {
	return instrumentation.NewMeasuredScrapperClient(pbv1.NewScrapperClient(conn))
}
