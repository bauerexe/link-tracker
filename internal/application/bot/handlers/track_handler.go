package handlers

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type TrackHandler struct {
	client pbv1.ScrapperClient
	ctx    context.Context
}

func NewTrackHandler(ctx context.Context, client pbv1.ScrapperClient) domain.Handler {
	return &TrackHandler{client: client, ctx: ctx}
}

func (h *TrackHandler) Handle(chatID int64, args string) (string, error) {
	parts := SplitArgs(args)
	if len(parts) == 0 {
		return "Некорректные данные", nil
	}

	link, ok := NormalizeURL(parts[0])
	if !ok {
		return "Некорректная ссылка. Пример: https://example.com ", nil
	}

	tags := []string(nil)
	if len(parts) > 1 {
		tags = parts[1:]
	}

	_, err := h.client.CreateLink(h.ctx, &pbv1.CreateLinkRequest{
		ChatId:  chatID,
		Link:    link,
		Tags:    tags,
		Filters: nil,
	}, grpc.WaitForReady(true))
	if err != nil {
		st, stOK := status.FromError(err)
		if stOK && st.Code() == codes.AlreadyExists {
			return "Ссылка уже отслеживается", nil
		}

		return "", fmt.Errorf("CreateLink failed: %w", err)
	}

	if len(tags) > 0 {
		return fmt.Sprintf("Начал отслеживать: %s\nТеги: %s", link, strings.Join(tags, ", ")), nil
	}
	return fmt.Sprintf("Начал отслеживать: %s", link), nil
}
