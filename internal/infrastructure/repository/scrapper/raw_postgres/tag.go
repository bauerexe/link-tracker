package raw_postgres

import (
	"context"
	"errors"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TagRepository struct {
	pool *pgxpool.Pool
}

func NewTagRepository(pool *pgxpool.Pool) usecase.TagRepository {
	return &TagRepository{pool: pool}
}

func (r *TagRepository) CreateTag(ctx context.Context, chatID int64, name string) error {
	var existingChatID int64
	err := r.pool.QueryRow(ctx, `
		SELECT id
		FROM chats
		WHERE id = $1
	`, chatID).Scan(&existingChatID)
	if err != nil {
		return usecase.ErrChatNotFound
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO tags (name)
		VALUES ($1)
	`, name)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return usecase.ErrTagAlreadyExist
		}
		return err
	}

	return nil
}

func (r *TagRepository) GetTagsByChatID(ctx context.Context, chatID int64) ([]string, error) {
	var existingChatID int64
	err := r.pool.QueryRow(ctx, `
		SELECT id
		FROM chats
		WHERE id = $1
	`, chatID).Scan(&existingChatID)
	if err != nil {
		return nil, usecase.ErrChatNotFound
	}

	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT t.name
		FROM tags t
		JOIN chat_link_tags clt ON clt.tag_id = t.id
		JOIN chat_links cl ON cl.id = clt.chat_link_id
		WHERE cl.chat_id = $1
		ORDER BY t.name
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var tag string
		if err = rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tags, nil
}

func (r *TagRepository) UpdateTag(ctx context.Context, chatID int64, oldName, newName string) error {
	var existingChatID int64
	err := r.pool.QueryRow(ctx, `
		SELECT id
		FROM chats
		WHERE id = $1
	`, chatID).Scan(&existingChatID)
	if err != nil {
		return usecase.ErrChatNotFound
	}

	var oldTagID int64
	err = r.pool.QueryRow(ctx, `
		SELECT DISTINCT t.id
		FROM tags t
		JOIN chat_link_tags clt ON clt.tag_id = t.id
		JOIN chat_links cl ON cl.id = clt.chat_link_id
		WHERE cl.chat_id = $1 AND t.name = $2
	`, chatID, oldName).Scan(&oldTagID)
	if err != nil {
		return usecase.ErrTagNotFound
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO tags (name)
		VALUES ($1)
		ON CONFLICT (name) DO NOTHING
	`, newName)
	if err != nil {
		return err
	}

	var newTagID int64
	err = r.pool.QueryRow(ctx, `
		SELECT id
		FROM tags
		WHERE name = $1
	`, newName).Scan(&newTagID)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE chat_link_tags clt
		SET tag_id = $1
		FROM chat_links cl
		WHERE clt.chat_link_id = cl.id
		  AND cl.chat_id = $2
		  AND clt.tag_id = $3
	`, newTagID, chatID, oldTagID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_, err = r.pool.Exec(ctx, `
				DELETE FROM chat_link_tags clt
				USING chat_links cl
				WHERE clt.chat_link_id = cl.id
				  AND cl.chat_id = $1
				  AND clt.tag_id = $2
			`, chatID, oldTagID)
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}

	return nil
}

func (r *TagRepository) DeleteTag(ctx context.Context, chatID int64, name string) error {
	var existingChatID int64
	err := r.pool.QueryRow(ctx, `
		SELECT id
		FROM chats
		WHERE id = $1
	`, chatID).Scan(&existingChatID)
	if err != nil {
		return usecase.ErrChatNotFound
	}

	tag, err := r.pool.Exec(ctx, `
		DELETE FROM chat_link_tags clt
		USING tags t, chat_links cl
		WHERE clt.tag_id = t.id
		  AND clt.chat_link_id = cl.id
		  AND cl.chat_id = $1
		  AND t.name = $2
	`, chatID, name)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return usecase.ErrTagNotFound
	}

	return nil
}
