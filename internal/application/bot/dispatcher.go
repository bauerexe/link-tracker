package botapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

const (
	CommandStart   domain.Command = "/start"
	CommandHelp    domain.Command = "/help"
	CommandTrack   domain.Command = "/track"
	CommandUntrack domain.Command = "/untrack"
	CommandList    domain.Command = "/list"
)

var ErrorUnknownCommand = errors.New("error unknown command")

func NewBotDispatcher(handlers map[domain.Command]domain.Handler) *BotDispatcher {
	if handlers == nil {
		handlers = map[domain.Command]domain.Handler{
			CommandStart: &StartHandler{},
			CommandHelp:  &HelpHandler{},
		}
	}
	return &BotDispatcher{handlers: handlers}
}

// BotDispatcher - dispatcher of commands and them handlers
type BotDispatcher struct {
	handlers map[domain.Command]domain.Handler
}

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

// HelpHandler - handler of message with command - CommandHelp
type HelpHandler struct{}

func (h *HelpHandler) Handle(_ int64, _ string) (string, error) {
	return "/start — начать\n" +
		"/help — список команд\n" +
		"/track — начать отслеживание ссылки. " +
		"Опционально пользователь может указать один или несколько тегов," +
		"привязанных к ссылке.\n" +
		"/untrack — прекратить отслеживание ссылки.\n" +
		"/list — вывести список всех ссылок," +
		"отслеживаемых пользователем." +
		"Опционально вторым параметром можно указать тег" +
		" — в этом случае список ссылок должен быть отфильтрован по указанному тегу.", nil
}

func NewHelpHandler() domain.Handler {
	return &HelpHandler{}
}

type TrackHandler struct {
	client pbv1.ScrapperClient
	ctx    context.Context
}

func NewTrackHandler(ctx context.Context, client pbv1.ScrapperClient) domain.Handler {
	return &TrackHandler{client: client, ctx: ctx}
}

func (h *TrackHandler) Handle(chatID int64, args string) (string, error) {
	parts := splitArgs(args)
	if len(parts) == 0 {
		return "Некорректные данные", nil
	}

	link, ok := normalizeURL(parts[0])
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
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.AlreadyExists {
			return "Ссылка уже отслеживается", nil
		}

		return "", fmt.Errorf("CreateLink failed: %w", err)
	}

	if len(tags) > 0 {
		return fmt.Sprintf("Начал отслеживать: %s\nТеги: %s", link, strings.Join(tags, ", ")), nil
	}
	return fmt.Sprintf("Начал отслеживать: %s", link), nil
}

type UntrackHandler struct {
	client pbv1.ScrapperClient
	ctx    context.Context
}

func NewUntrackHandler(ctx context.Context, client pbv1.ScrapperClient) domain.Handler {
	return &UntrackHandler{client: client, ctx: ctx}
}

func (h *UntrackHandler) Handle(chatID int64, args string) (string, error) {
	parts := splitArgs(args)
	if len(parts) == 0 {
		return "Использование: /untrack <ссылка>", nil
	}

	link, ok := normalizeURL(parts[0])
	if !ok {
		return "Некорректная ссылка. Пример: /untrack https://example.com", nil
	}

	_, err := h.client.DeleteLink(h.ctx, &pbv1.DeleteLinkRequest{
		ChatId: chatID,
		Link:   link,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return "Ссылка не отслеживается", nil
		}
		return "", fmt.Errorf("DeleteLink failed: %w", err)
	}

	return fmt.Sprintf("Перестал отслеживать: %s", link), nil
}

type ListHandler struct {
	client pbv1.ScrapperClient
	ctx    context.Context
}

func NewListHandler(ctx context.Context, client pbv1.ScrapperClient) domain.Handler {
	return &ListHandler{client: client, ctx: ctx}
}

func (h *ListHandler) Handle(chatID int64, args string) (string, error) {
	parts := splitArgs(args)
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
			if hasTag(l.GetTags(), tag) {
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

// Dispatch - return of the handler's work
func (r *BotDispatcher) Dispatch(message domain.Message, cmd domain.Command, args string) (string, error) {
	h, ok := r.handlers[cmd]
	if !ok {
		return fmt.Sprintf("Не знаю команду %s. Напиши /help", cmd), nil
	}
	return h.Handle(message.ChatID, args)
}
