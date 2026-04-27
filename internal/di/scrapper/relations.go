package scrapper

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

var GRPCModule = fx.Options(
	fx.Provide(
		newBotConn,
		newBotClient,
	),
)

func newBotConn(
	lc fx.Lifecycle,
	cfg *config.ScrapperConfig,
	log *zap.Logger,
) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		cfg.BotAddrGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
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

var KafkaModule = fx.Options(
	fx.Provide(
		newCfgSarama,
	),
)

func newCfgSarama() (*sarama.Config, error) {
	cfg := kafka.New()
	return cfg, nil
}
