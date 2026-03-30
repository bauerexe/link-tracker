package repository

import (
	"context"
	"fmt"
	"sync"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type ChatRepository struct {
	chats map[int64]*domain.Chat
	mu    sync.RWMutex
}

// NewChatRepository - return inmemory implementation of scrapper.ChatRepository
func NewChatRepository() usecase.ChatRepository {
	return &ChatRepository{chats: make(map[int64]*domain.Chat), mu: sync.RWMutex{}}
}

func (c *ChatRepository) CreateChat(ctx context.Context, id int64) (*domain.Chat, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("create chat: context done: %w", ctx.Err())
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if chat, ok := c.chats[id]; ok {
		return chat, usecase.ErrChatAlreadyExist
	}
	newChat := domain.Chat{ID: id}
	c.chats[id] = &newChat
	return &newChat, nil
}

func (c *ChatRepository) GetChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("get chat by id: context done: %w", ctx.Err())
	default:
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	if chat, ok := c.chats[id]; ok {
		return chat, nil
	}
	return nil, usecase.ErrChatNotFound
}

func (c *ChatRepository) DeleteChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("delete chat by id: context done: %w", ctx.Err())
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if chat, ok := c.chats[id]; ok {
		delete(c.chats, id)
		return chat, nil
	}
	return nil, usecase.ErrChatNotFound
}
