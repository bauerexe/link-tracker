package bot_gateway

import (
	"context"

	"go.uber.org/zap"

	botapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type dummyGateway struct {
	log *zap.Logger
}

func (d dummyGateway) GetMessages(ctx context.Context, _ int) (<-chan domain.Message, error) {
	ch := make(chan domain.Message)

	go func() {
		<-ctx.Done()
		close(ch)
	}()

	return ch, nil
}

func (d dummyGateway) SendMessage(_ int64, _ int, _ string) error {
	return nil
}

func NewDummy(log *zap.Logger) botapp.BotGateway {
	return &dummyGateway{log: log}
}
