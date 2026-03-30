package postgres

import (
	"context"
	"errors"
	"fmt"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatRepository struct {
	pool    *pgxpool.Pool
	dialect goqu.DialectWrapper
}

func NewChatRepository(pool *pgxpool.Pool) usecase.ChatRepository {
	return &ChatRepository{
		pool:    pool,
		dialect: goqu.Dialect("postgres"),
	}
}

func (r *ChatRepository) CreateChat(ctx context.Context, id int64) (*domain.Chat, error) {
	sql, _, err := r.dialect.Insert("chats").
		Rows(goqu.Record{
			"id": id,
		}).
		Returning("id").
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build create chat query: %w", err)
	}

	var chatID int64
	err = r.pool.QueryRow(ctx, sql).Scan(&chatID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, usecase.ErrChatAlreadyExist
		}
		return nil, fmt.Errorf("scan created chat id: %w", err)
	}

	return &domain.Chat{ID: chatID}, nil
}

func (r *ChatRepository) GetChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	sql, _, err := r.dialect.From("chats").
		Select("id").
		Where(goqu.C("id").Eq(id)).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build get chat query: %w", err)
	}

	var chatID int64
	err = r.pool.QueryRow(ctx, sql).Scan(&chatID)
	if err != nil {
		return nil, usecase.ErrChatNotFound
	}

	return &domain.Chat{ID: chatID}, nil
}

func (r *ChatRepository) DeleteChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	sql, _, err := r.dialect.Delete("chats").
		Where(goqu.C("id").Eq(id)).
		Returning("id").
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build delete chat query: %w", err)
	}

	var chatID int64
	err = r.pool.QueryRow(ctx, sql).Scan(&chatID)
	if err != nil {
		return nil, usecase.ErrChatNotFound
	}

	return &domain.Chat{ID: chatID}, nil
}
