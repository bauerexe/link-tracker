package repository

import (
	"context"
	"sync"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type LinkRepository struct {
	mu    sync.RWMutex
	links map[int64]map[string]*domain.Link
	curID int64
}

func NewLinkRepository() usecase.LinkRepository {
	return &LinkRepository{
		links: make(map[int64]map[string]*domain.Link),
	}
}

func (l *LinkRepository) nextID() int64 {
	l.curID++
	return l.curID
}

func (l *LinkRepository) CreateLink(
	ctx context.Context, chatID int64, url string,
	tags, filters []string,
) (*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.links[chatID]; !ok {
		l.links[chatID] = make(map[string]*domain.Link)
	}

	if _, exists := l.links[chatID][url]; exists {
		return nil, usecase.ErrLinkAlreadyTracked
	}

	link := &domain.Link{
		URL:     url,
		Tags:    append([]string(nil), tags...),
		Filters: append([]string(nil), filters...),
		ID:      l.nextID(),
	}

	l.links[chatID][url] = link
	return link, nil
}

func (l *LinkRepository) GetLinksByChatID(ctx context.Context, chatID int64) ([]*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	chatLinks, ok := l.links[chatID]
	if !ok {
		return []*domain.Link{}, usecase.ErrChatNotFound
	}

	result := make([]*domain.Link, 0, len(chatLinks))
	for _, link := range chatLinks {
		result = append(result, link)
	}

	return result, nil
}

func (l *LinkRepository) DeleteLink(ctx context.Context, chatID int64, url string) (*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	chatLinks, ok := l.links[chatID]
	if !ok {
		return nil, usecase.ErrChatNotFound
	}

	if _, exists := chatLinks[url]; !exists {
		return nil, usecase.ErrLinkNotFound
	}

	deleted := chatLinks[url]
	delete(chatLinks, url)

	if len(chatLinks) == 0 {
		delete(l.links, chatID)
	}

	return deleted, nil
}
