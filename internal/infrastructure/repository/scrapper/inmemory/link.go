package repository

import (
	"context"
	"sort"
	"sync"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type LinkRepository struct {
	mu         sync.RWMutex
	idUrlLinks map[int64]map[string]*domain.Link
	urlIdLinks map[string]map[int64]*domain.Link
	urlState   map[string]domain.URLState
	curID      int64
}

// NewLinkRepository - return inmemory implementation of scrapper.LinkRepository
func NewLinkRepository() usecase.LinkRepository {
	return &LinkRepository{
		idUrlLinks: make(map[int64]map[string]*domain.Link),
		urlIdLinks: make(map[string]map[int64]*domain.Link),
		urlState:   make(map[string]domain.URLState),
	}
}

func (l *LinkRepository) nextID() int64 {
	l.curID++
	return l.curID
}

func (l *LinkRepository) CreateLink(
	ctx context.Context,
	chatID int64,
	url string,
	tags, filters []string,
) (*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.idUrlLinks[chatID]; !ok {
		l.idUrlLinks[chatID] = make(map[string]*domain.Link)
	}

	if _, exists := l.idUrlLinks[chatID][url]; exists {
		return nil, usecase.ErrLinkAlreadyTracked
	}

	link := &domain.Link{
		ID:      l.nextID(),
		URL:     url,
		Tags:    append([]string(nil), tags...),
		Filters: append([]string(nil), filters...),
	}

	l.idUrlLinks[chatID][url] = link

	if _, ok := l.urlIdLinks[url]; !ok {
		l.urlIdLinks[url] = make(map[int64]*domain.Link)
	}
	l.urlIdLinks[url][chatID] = link

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

	chatLinks, ok := l.idUrlLinks[chatID]
	if !ok {
		return []*domain.Link{}, usecase.ErrChatNotFound
	}

	urls := make([]string, 0, len(chatLinks))
	for url := range chatLinks {
		urls = append(urls, url)
	}
	sort.Strings(urls)

	res := make([]*domain.Link, 0, len(chatLinks))
	for _, url := range urls {
		res = append(res, chatLinks[url])
	}

	return res, nil
}

func (l *LinkRepository) DeleteLink(ctx context.Context, chatID int64, url string) (*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	chatLinks, ok := l.idUrlLinks[chatID]
	if !ok {
		return nil, usecase.ErrChatNotFound
	}

	link, exists := chatLinks[url]
	if !exists {
		return nil, usecase.ErrLinkNotFound
	}

	delete(chatLinks, url)
	if len(chatLinks) == 0 {
		delete(l.idUrlLinks, chatID)
	}

	if m, ok := l.urlIdLinks[url]; ok {
		delete(m, chatID)
		if len(m) == 0 {
			delete(l.urlIdLinks, url)
			delete(l.urlState, url)
		}
	}

	return link, nil
}

func (l *LinkRepository) ListLinks(ctx context.Context) ([]*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	urls := make([]string, 0, len(l.urlIdLinks))
	for url := range l.urlIdLinks {
		urls = append(urls, url)
	}
	sort.Strings(urls)

	res := make([]*domain.Link, 0, len(urls))
	for _, url := range urls {
		res = append(res, &domain.Link{
			URL: url,
		})
	}

	return res, nil
}

func (l *LinkRepository) GetChatIDsByLink(ctx context.Context, url string) ([]int64, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	m, ok := l.urlIdLinks[url]
	if !ok || len(m) == 0 {
		return nil, usecase.ErrLinkNotFound
	}

	ids := make([]int64, 0, len(m))
	for chatID := range m {
		ids = append(ids, chatID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	return ids, nil
}

func (l *LinkRepository) GetURLState(ctx context.Context, url string) (domain.URLState, error) {
	select {
	case <-ctx.Done():
		return domain.URLState{}, ctx.Err()
	default:
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	st, ok := l.urlState[url]
	if !ok {
		return domain.URLState{}, nil
	}
	return st, nil
}

func (l *LinkRepository) SetURLState(ctx context.Context, url string, st domain.URLState) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.urlIdLinks[url]; !ok {
		return usecase.ErrLinkNotFound
	}

	l.urlState[url] = st
	return nil
}
