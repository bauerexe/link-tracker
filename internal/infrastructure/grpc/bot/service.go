package botcontroller

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/observability"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

// Messenger - interface that need for send message, when server got request
type Messenger interface {
	SendMessage(chatID int64, replyToMessageID int, text string) error
}

type api struct {
	msg Messenger
	log *zap.Logger
}

// New - return implementation of pbv1.BotServer
func New(log *zap.Logger, msg Messenger) pbv1.BotServer {
	return &api{
		msg: msg,
		log: log,
	}
}

func (a *api) UpdateLink(_ context.Context, req *pbv1.UpdateLinkRequest) (*pbv1.UpdateLinkResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid argument: %w", status.Error(codes.InvalidArgument, "empty request"))
	}
	if len(req.GetTgChatIds()) == 0 {
		a.log.Debug("tgChatsIds is nil")
		return nil, fmt.Errorf("invalid argument: %w", status.Error(codes.InvalidArgument, "empty request"))
	}

	text := formatUpdate(req)

	var failed []string
	for _, chatID := range req.GetTgChatIds() {
		if err := a.msg.SendMessage(chatID, 0, text); err != nil {
			a.log.Debug("can`t send message", zap.Int64("chatId", chatID))
			failed = append(failed, strconv.FormatInt(chatID, 10))
			continue
		}
		observability.IncSentNotification()
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
