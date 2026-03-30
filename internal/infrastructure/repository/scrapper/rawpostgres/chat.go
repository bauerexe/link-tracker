package rawpostgres

import (
	"context"
	"errors"
	"fmt"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	CreateChatQuery = `
		INSERT INTO chats (id)
		VALUES ($1)
		RETURNING id
	`
	GetChatByIDQuery = `
		SELECT id
		FROM chats
		WHERE id = $1
	`
	DeleteChatByIDQuery = `
		DELETE FROM chats
		WHERE id = $1
		RETURNING id
	`
)

type ChatRepository struct {
	pool *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) usecase.ChatRepository {
	return &ChatRepository{pool: pool}
}

func (r *ChatRepository) CreateChat(ctx context.Context, id int64) (*domain.Chat, error) {
	var chatID int64

	err := r.pool.QueryRow(ctx, CreateChatQuery, id).Scan(&chatID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, usecase.ErrChatAlreadyExist
		}
		return nil, fmt.Errorf("create chat: %w", err)
	}

	return &domain.Chat{ID: chatID}, nil
}

func (r *ChatRepository) GetChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	var chatID int64

	err := r.pool.QueryRow(ctx, GetChatByIDQuery, id).Scan(&chatID)
	if err != nil {
		return nil, usecase.ErrChatNotFound
	}

	return &domain.Chat{ID: chatID}, nil
}

func (r *ChatRepository) DeleteChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	var chatID int64

	err := r.pool.QueryRow(ctx, DeleteChatByIDQuery, id).Scan(&chatID)
	if err != nil {
		return nil, usecase.ErrChatNotFound
	}

	return &domain.Chat{ID: chatID}, nil
}
