package scrapper_app

import (
	"context"

	"go.uber.org/zap"

	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type GRPCBotNotifier struct {
	client pbv1.BotClient
	log    *zap.Logger
}

func NewGRPCBotNotifier(client pbv1.BotClient, log *zap.Logger) *GRPCBotNotifier {
	if log != nil {
		log = log.Named("bot_notifier")
	}
	return &GRPCBotNotifier{client: client, log: log}
}

func (n *GRPCBotNotifier) Notify(ctx context.Context, url, description string, chatIDs []int64) error {
	_, err := n.client.UpdateLink(ctx, &pbv1.UpdateLinkRequest{
		Url:         url,
		Description: description,
		TgChatIds:   chatIDs,
	})
	if err != nil && n.log != nil {
		n.log.Error("UpdateLink failed", zap.Error(err))
	}
	return err
}
