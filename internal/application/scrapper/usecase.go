package scrapper_app

import (
	"context"
	"errors"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

//go:generate mockgen -source usecase.go -package scrapper_app -destination usecase_mock.go
var (
	ErrChatAlreadyExist = errors.New("err: chat with same id already exist")
	ErrChatNotFound     = errors.New("err: chat does not found")

	ErrLinkAlreadyTracked = errors.New("err: link with same url already tracked")
	ErrLinkNotFound       = errors.New("err: link does not found")
	ErrInvalidParams      = errors.New("err: invalid params")
)

type ChatRepository interface {
	CreateChat(ctx context.Context, ID int64) (*domain.Chat, error)
	GetChatByID(ctx context.Context, ID int64) (*domain.Chat, error)
	DeleteChatByID(ctx context.Context, ID int64) (*domain.Chat, error)
}

type LinkRepository interface {
	CreateLink(ctx context.Context, ChatID int64, URL string, Tags, Filters []string) (*domain.Link, error)
	GetLinksByChatID(ctx context.Context, ChatID int64) ([]*domain.Link, error)
	DeleteLink(ctx context.Context, ChatID int64, URL string) (*domain.Link, error)
}
