package bot_controller

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type Messenger interface {
	SendMessage(chatID int64, replyToMessageID int, text string) error
}

type api struct {
	log *zap.Logger
	msg Messenger
}

func New(log *zap.Logger, msg Messenger) pbv1.BotServer {
	return &api{
		log: log.Named("bot_api"),
		msg: msg,
	}
}

func (a *api) UpdateLink(_ context.Context, req *pbv1.UpdateLinkRequest) (*pbv1.UpdateLinkResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	if len(req.GetTgChatIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "tgChatIds must not be empty")
	}

	text := formatUpdate(req)

	var failed []string
	for _, chatID := range req.GetTgChatIds() {
		if err := a.msg.SendMessage(chatID, 0, text); err != nil {
			a.log.Error("send message failed", zap.Int64("chat_id", chatID), zap.Error(err))
			failed = append(failed, fmt.Sprintf("%d", chatID))
		}
	}

	if len(failed) > 0 {
		return nil, status.Errorf(codes.Internal, "failed to send to chats: %s", strings.Join(failed, ","))
	}

	return &pbv1.UpdateLinkResponse{}, nil
}

func formatUpdate(req *pbv1.UpdateLinkRequest) string {
	if req.GetDescription() != "" {
		return fmt.Sprintf("🔔 Обновление по ссылке:\n%s\n\n%s", req.GetUrl(), req.GetDescription())
	}
	return fmt.Sprintf("🔔 Обновление по ссылке:\n%s", req.GetUrl())
}
