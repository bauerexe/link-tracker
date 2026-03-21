package handlers

import (
	"context"
	"fmt"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

// StartHandler - handler of message with command - CommandStart
type StartHandler struct {
	ctx    context.Context
	client pbv1.ScrapperClient
}

func NewStartHandler(ctx context.Context, client pbv1.ScrapperClient) domain.Handler {
	return &StartHandler{
		ctx:    ctx,
		client: client,
	}
}

func (h *StartHandler) Handle(chatID int64, _ string) (string, error) {
	_, err := h.client.CreateChat(h.ctx, &pbv1.CreateChatRequest{
		Id: chatID,
	})
	if err != nil {
		return "", fmt.Errorf("CreateChat failed: %w", err)
	}

	return "Привет! Я link-tracker бот. Напиши /help", nil
}
