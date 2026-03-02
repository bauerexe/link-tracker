package repository

import (
	"context"
	"sync"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type ChatRepository struct {
	chats map[int64]*domain.Chat
	mu    sync.RWMutex
}

func NewChatRepository() usecase.ChatRepository {
	return &ChatRepository{chats: make(map[int64]*domain.Chat), mu: sync.RWMutex{}}
}

func (c *ChatRepository) CreateChat(ctx context.Context, ID int64) (*domain.Chat, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if chat, ok := c.chats[ID]; ok {
		return chat, usecase.ErrChatAlreadyExist
	}
	newChat := domain.Chat{ID: ID}
	c.chats[ID] = &newChat
	return &newChat, nil
}

func (c *ChatRepository) GetChatByID(ctx context.Context, ID int64) (*domain.Chat, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	if chat, ok := c.chats[ID]; ok {
		return chat, nil
	}
	return nil, usecase.ErrChatNotFound
}

func (c *ChatRepository) DeleteChatByID(ctx context.Context, ID int64) (*domain.Chat, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if chat, ok := c.chats[ID]; ok {
		delete(c.chats, ID)
		return chat, nil
	}
	return nil, usecase.ErrChatNotFound
}
