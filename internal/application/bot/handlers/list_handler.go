package handlers

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type ListHandler struct {
	client pbv1.ScrapperClient
	ctx    context.Context
}

func NewListHandler(ctx context.Context, client pbv1.ScrapperClient) domain.Handler {
	return &ListHandler{client: client, ctx: ctx}
}

func (h *ListHandler) Handle(chatID int64, args string) (string, error) {
	parts := SplitArgs(args)
	var tag string
	if len(parts) > 0 {
		tag = parts[0]
	}

	resp, err := h.client.GetLinks(h.ctx, &pbv1.GetLinksRequest{
		ChatId: chatID,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return "Список отслеживаемых ссылок пуст.", nil
		}
		return "", fmt.Errorf("GetLinks failed: %w", err)
	}

	links := resp.GetLinks()
	if len(links) == 0 {
		return "Список отслеживаемых ссылок пуст.", nil
	}

	filtered := make([]*pbv1.LinkResponse, 0, len(links))
	if tag != "" {
		for _, l := range links {
			if HasTag(l.GetTags(), tag) {
				filtered = append(filtered, l)
			}
		}
	} else {
		filtered = links
	}

	if len(filtered) == 0 {
		return fmt.Sprintf("По тегу %q ничего не найдено.", tag), nil
	}

	var b strings.Builder
	if tag != "" {
		b.WriteString(fmt.Sprintf("Ссылки с тегом %q:\n", tag))
	} else {
		b.WriteString("Отслеживаемые ссылки:\n")
	}

	for i, l := range filtered {
		b.WriteString(fmt.Sprintf("%d) %s\n", i+1, l.GetUrl()))
	}

	return strings.TrimRight(b.String(), "\n"), nil
}
