package handlers

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type UntrackHandler struct {
	client pbv1.ScrapperClient
	ctx    context.Context
}

func NewUntrackHandler(ctx context.Context, client pbv1.ScrapperClient) domain.Handler {
	return &UntrackHandler{client: client, ctx: ctx}
}

func (h *UntrackHandler) Handle(chatID int64, args string) (string, error) {
	parts := SplitArgs(args)
	if len(parts) == 0 {
		return "Использование: /untrack <ссылка>", nil
	}

	link, ok := NormalizeURL(parts[0])
	if !ok {
		return "Некорректная ссылка. Пример: /untrack https://example.com", nil
	}

	_, err := h.client.DeleteLink(h.ctx, &pbv1.DeleteLinkRequest{
		ChatId: chatID,
		Link:   link,
	})
	if err != nil {
		var st *status.Status
		st, ok = status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return "Ссылка не отслеживается", nil
		}
		return "", fmt.Errorf("DeleteLink failed: %w", err)
	}

	return fmt.Sprintf("Перестал отслеживать: %s", link), nil
}
