package scrapperapp

import (
	"context"
	"errors"
	"time"

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

	ListLinks(ctx context.Context) ([]*domain.Link, error)
	GetChatIDsByLink(ctx context.Context, url string) ([]int64, error)

	GetURLState(ctx context.Context, url string) (domain.URLState, error)
	SetURLState(ctx context.Context, url string, st domain.URLState) error
}

type BotNotifier interface {
	Notify(ctx context.Context, url, description string, chatIDs []int64) error
}

type Checker interface {
	Match(url string) bool
	Check(ctx context.Context, url string, since time.Time) (desc string, updatedAt time.Time, updated bool, err error)
}
