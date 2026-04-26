package scrapperapp

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
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
		if n.log != nil {
			n.log.Error("UpdateLink failed", zap.Error(err))
		}
		return fmt.Errorf("notify err: %w", err)
	}
	return nil
}

type KafkaBotNotifier struct {
	producer *kafka.ProducerScrapperToBot
	log      *zap.Logger
}

func NewKafkaBotNotifier(producer *kafka.ProducerScrapperToBot, log *zap.Logger) *KafkaBotNotifier {
	if log != nil {
		log = log.Named("bot_notifier")
	}
	return &KafkaBotNotifier{producer: producer, log: log}
}

func (n *KafkaBotNotifier) Notify(_ context.Context, url, description string, chatIDs []int64) error {
	msg := &pbv1.UpdateLinkRequest{
		Url:         url,
		Description: description,
		TgChatIds:   chatIDs,
	}

	value, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal updateLinkRequest failed: %w", err)
	}

	n.producer.Messages <- &sarama.ProducerMessage{
		Topic: n.producer.CfgKafka.KafkaTopic,
		Value: sarama.ByteEncoder(value),
	}
	return nil
}
