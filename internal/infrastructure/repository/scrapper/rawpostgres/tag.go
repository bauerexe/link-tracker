package rawpostgres

import (
	"context"
	"errors"
	"fmt"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	CreateTagSelectChatQuery = `
		SELECT id
		FROM chats
		WHERE id = $1
	`
	CreateTagInsertQuery = `
		INSERT INTO tags (name)
		VALUES ($1)
	`

	GetTagsByChatIDSelectChatQuery = `
		SELECT id
		FROM chats
		WHERE id = $1
	`
	GetTagsByChatIDSelectTagsQuery = `
		SELECT DISTINCT t.name
		FROM tags t
		JOIN chat_link_tags clt ON clt.tag_id = t.id
		JOIN chat_links cl ON cl.id = clt.chat_link_id
		WHERE cl.chat_id = $1
		ORDER BY t.name
	`

	UpdateTagSelectChatQuery = `
		SELECT id
		FROM chats
		WHERE id = $1
	`
	UpdateTagSelectOldTagIDQuery = `
		SELECT DISTINCT t.id
		FROM tags t
		JOIN chat_link_tags clt ON clt.tag_id = t.id
		JOIN chat_links cl ON cl.id = clt.chat_link_id
		WHERE cl.chat_id = $1 AND t.name = $2
	`
	UpdateTagInsertNewTagQuery = `
		INSERT INTO tags (name)
		VALUES ($1)
		ON CONFLICT (name) DO NOTHING
	`
	UpdateTagSelectNewTagIDQuery = `
		SELECT id
		FROM tags
		WHERE name = $1
	`
	UpdateTagUpdateChatLinkTagsQuery = `
		UPDATE chat_link_tags clt
		SET tag_id = $1
		FROM chat_links cl
		WHERE clt.chat_link_id = cl.id
		  AND cl.chat_id = $2
		  AND clt.tag_id = $3
	`
	UpdateTagDeleteOldChatLinkTagsQuery = `
		DELETE FROM chat_link_tags clt
		USING chat_links cl
		WHERE clt.chat_link_id = cl.id
		  AND cl.chat_id = $1
		  AND clt.tag_id = $2
	`

	DeleteTagSelectChatQuery = `
		SELECT id
		FROM chats
		WHERE id = $1
	`
	DeleteTagDeleteQuery = `
		DELETE FROM chat_link_tags clt
		USING tags t, chat_links cl
		WHERE clt.tag_id = t.id
		  AND clt.chat_link_id = cl.id
		  AND cl.chat_id = $1
		  AND t.name = $2
	`
)

type TagRepository struct {
	pool *pgxpool.Pool
}

func NewTagRepository(pool *pgxpool.Pool) usecase.TagRepository {
	return &TagRepository{pool: pool}
}

func (r *TagRepository) CreateTag(ctx context.Context, chatID int64, name string) error {
	var existingChatID int64
	scanErr := r.pool.QueryRow(ctx, CreateTagSelectChatQuery, chatID).Scan(&existingChatID)
	if scanErr != nil {
		return usecase.ErrChatNotFound
	}

	_, err := r.pool.Exec(ctx, CreateTagInsertQuery, name)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return usecase.ErrTagAlreadyExist
		}
		return fmt.Errorf("insert tag: %w", err)
	}

	return nil
}

func (r *TagRepository) GetTagsByChatID(ctx context.Context, chatID int64) ([]string, error) {
	var existingChatID int64
	scanErr := r.pool.QueryRow(ctx, GetTagsByChatIDSelectChatQuery, chatID).Scan(&existingChatID)
	if scanErr != nil {
		return nil, usecase.ErrChatNotFound
	}

	rows, err := r.pool.Query(ctx, GetTagsByChatIDSelectTagsQuery, chatID)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var tag string
		rowErr := rows.Scan(&tag)
		if rowErr != nil {
			return nil, fmt.Errorf("scan tag: %w", rowErr)
		}
		tags = append(tags, tag)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("rows error: %w", rowsErr)
	}

	return tags, nil
}

func (r *TagRepository) UpdateTag(ctx context.Context, chatID int64, oldName, newName string) error {
	var existingChatID int64
	scanErr := r.pool.QueryRow(ctx, UpdateTagSelectChatQuery, chatID).Scan(&existingChatID)
	if scanErr != nil {
		return usecase.ErrChatNotFound
	}

	var oldTagID int64
	scanErr = r.pool.QueryRow(ctx, UpdateTagSelectOldTagIDQuery, chatID, oldName).Scan(&oldTagID)
	if scanErr != nil {
		return usecase.ErrTagNotFound
	}

	_, err := r.pool.Exec(ctx, UpdateTagInsertNewTagQuery, newName)
	if err != nil {
		return fmt.Errorf("insert new tag: %w", err)
	}

	var newTagID int64
	scanErr = r.pool.QueryRow(ctx, UpdateTagSelectNewTagIDQuery, newName).Scan(&newTagID)
	if scanErr != nil {
		return fmt.Errorf("select new tag id: %w", scanErr)
	}

	_, err = r.pool.Exec(ctx, UpdateTagUpdateChatLinkTagsQuery, newTagID, chatID, oldTagID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_, delErr := r.pool.Exec(ctx, UpdateTagDeleteOldChatLinkTagsQuery, chatID, oldTagID)
			if delErr != nil {
				return fmt.Errorf("delete old tag links: %w", delErr)
			}
			return nil
		}
		return fmt.Errorf("update tag: %w", err)
	}

	return nil
}

func (r *TagRepository) DeleteTag(ctx context.Context, chatID int64, name string) error {
	var existingChatID int64
	scanErr := r.pool.QueryRow(ctx, DeleteTagSelectChatQuery, chatID).Scan(&existingChatID)
	if scanErr != nil {
		return usecase.ErrChatNotFound
	}

	tag, err := r.pool.Exec(ctx, DeleteTagDeleteQuery, chatID, name)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return usecase.ErrTagNotFound
	}

	return nil
}
