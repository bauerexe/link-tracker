package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type LinkRepository struct {
	mu         sync.RWMutex
	idURLLinks map[int64]map[string]*domain.Link
	urlIDLinks map[string]map[int64]*domain.Link
	urlState   map[string]domain.URLState
	curID      int64
}

// NewLinkRepository - return inmemory implementation of scrapper.LinkRepository
func NewLinkRepository() usecase.LinkRepository {
	return &LinkRepository{
		idURLLinks: make(map[int64]map[string]*domain.Link),
		urlIDLinks: make(map[string]map[int64]*domain.Link),
		urlState:   make(map[string]domain.URLState),
	}
}

func (l *LinkRepository) CreateLink(
	ctx context.Context,
	chatID int64,
	url string,
	tags, filters []string,
) (*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("create link: context done: %w", ctx.Err())
	default:
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.idURLLinks[chatID]; !ok {
		l.idURLLinks[chatID] = make(map[string]*domain.Link)
	}

	if _, exists := l.idURLLinks[chatID][url]; exists {
		return nil, usecase.ErrLinkAlreadyTracked
	}

	link := &domain.Link{
		ID:      l.nextID(),
		URL:     url,
		Tags:    append([]string(nil), tags...),
		Filters: append([]string(nil), filters...),
	}

	l.idURLLinks[chatID][url] = link

	if _, ok := l.urlIDLinks[url]; !ok {
		l.urlIDLinks[url] = make(map[int64]*domain.Link)
	}
	l.urlIDLinks[url][chatID] = link

	return link, nil
}

func (l *LinkRepository) GetLinksByChatID(ctx context.Context, chatID int64, limit, offset uint64) ([]*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("get links by chat id: context done: %w", ctx.Err())
	default:
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	chatLinks, ok := l.idURLLinks[chatID]
	if !ok {
		return nil, usecase.ErrChatNotFound
	}

	urls := make([]string, 0, len(chatLinks))
	for url := range chatLinks {
		urls = append(urls, url)
	}
	sort.Strings(urls)

	if offset >= uint64(len(urls)) {
		return []*domain.Link{}, nil
	}

	end := offset + limit
	if end > uint64(len(urls)) {
		end = uint64(len(urls))
	}

	res := make([]*domain.Link, 0, end-offset)
	for _, url := range urls[offset:end] {
		res = append(res, chatLinks[url])
	}

	return res, nil
}

func (l *LinkRepository) DeleteLink(ctx context.Context, chatID int64, url string) (*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("delete link: context done: %w", ctx.Err())
	default:
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	chatLinks, ok := l.idURLLinks[chatID]
	if !ok {
		return nil, usecase.ErrChatNotFound
	}

	link, exists := chatLinks[url]
	if !exists {
		return nil, usecase.ErrLinkNotFound
	}

	delete(chatLinks, url)
	if len(chatLinks) == 0 {
		delete(l.idURLLinks, chatID)
	}

	m, existsURL := l.urlIDLinks[url]
	if existsURL {
		delete(m, chatID)
		if len(m) == 0 {
			delete(l.urlIDLinks, url)
			delete(l.urlState, url)
		}
	}

	return link, nil
}

func (l *LinkRepository) ListLinksBatch(ctx context.Context, limit, offset int) ([]*domain.Link, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("list links batch: context done: %w", ctx.Err())
	default:
	}

	if limit <= 0 {
		return []*domain.Link{}, nil
	}
	if offset < 0 {
		return nil, errors.New("invalid offset")
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	urls := make([]string, 0, len(l.urlIDLinks))
	for url := range l.urlIDLinks {
		urls = append(urls, url)
	}
	sort.Strings(urls)

	if offset >= len(urls) {
		return []*domain.Link{}, nil
	}

	end := offset + limit
	if end > len(urls) {
		end = len(urls)
	}

	res := make([]*domain.Link, 0, end-offset)
	for i := offset; i < end; i++ {
		url := urls[i]
		res = append(res, &domain.Link{
			URL: url,
		})
	}

	return res, nil
}

func (l *LinkRepository) GetChatIDsByLink(ctx context.Context, url string) ([]int64, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("get chat ids by link: context done: %w", ctx.Err())
	default:
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	m, ok := l.urlIDLinks[url]
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
		return domain.URLState{}, fmt.Errorf("get url state: context done: %w", ctx.Err())
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
		return fmt.Errorf("set url state: context done: %w", ctx.Err())
	default:
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.urlIDLinks[url]; !ok {
		return usecase.ErrLinkNotFound
	}

	l.urlState[url] = st
	return nil
}

func (l *LinkRepository) nextID() int64 {
	l.curID++
	return l.curID
}
