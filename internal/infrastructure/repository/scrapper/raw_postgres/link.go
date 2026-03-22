package raw_postgres

import (
	"context"
	"errors"
	"time"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LinkRepository struct {
	pool *pgxpool.Pool
}

func NewLinkRepository(pool *pgxpool.Pool) usecase.LinkRepository {
	return &LinkRepository{pool: pool}
}

func (r *LinkRepository) CreateLink(
	ctx context.Context,
	chatID int64,
	url string,
	tags, filters []string,
) (*domain.Link, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO links (url)
		VALUES ($1)
		ON CONFLICT (url) DO NOTHING
	`, url)
	if err != nil {
		return nil, err
	}

	var linkID int64
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM links
		WHERE url = $1
	`, url).Scan(&linkID)
	if err != nil {
		return nil, err
	}

	var chatLinkID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO chat_links (chat_id, link_id)
		VALUES ($1, $2)
		RETURNING id
	`, chatID, linkID).Scan(&chatLinkID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23503" {
				return nil, usecase.ErrChatNotFound
			}
			if pgErr.Code == "23505" {
				return nil, usecase.ErrLinkAlreadyTracked
			}
		}
		return nil, err
	}

	for _, tag := range tags {
		_, err = tx.Exec(ctx, `
			INSERT INTO tags (name)
			VALUES ($1)
			ON CONFLICT (name) DO NOTHING
		`, tag)
		if err != nil {
			return nil, err
		}

		var tagID int64
		err = tx.QueryRow(ctx, `
			SELECT id
			FROM tags
			WHERE name = $1
		`, tag).Scan(&tagID)
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO chat_link_tags (chat_link_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT (chat_link_id, tag_id) DO NOTHING
		`, chatLinkID, tagID)
		if err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &domain.Link{
		ID:      chatLinkID,
		URL:     url,
		Tags:    append([]string(nil), tags...),
		Filters: append([]string(nil), filters...),
	}, nil
}

func (r *LinkRepository) GetLinksByChatID(ctx context.Context, chatID int64) ([]*domain.Link, error) {
	var existingChatID int64
	err := r.pool.QueryRow(ctx, `
		SELECT id
		FROM chats
		WHERE id = $1
	`, chatID).Scan(&existingChatID)
	if err != nil {
		return []*domain.Link{}, usecase.ErrChatNotFound
	}

	rows, err := r.pool.Query(ctx, `
		SELECT cl.id, l.url
		FROM chat_links cl
		JOIN links l ON l.id = cl.link_id
		WHERE cl.chat_id = $1
		ORDER BY l.url
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]*domain.Link, 0)
	for rows.Next() {
		var chatLinkID int64
		var url string

		if err = rows.Scan(&chatLinkID, &url); err != nil {
			return nil, err
		}

		tags, err := r.getTagsByChatLinkID(ctx, chatLinkID)
		if err != nil {
			return nil, err
		}

		res = append(res, &domain.Link{
			ID:      chatLinkID,
			URL:     url,
			Tags:    tags,
			Filters: []string{},
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}

func (r *LinkRepository) DeleteLink(ctx context.Context, chatID int64, url string) (*domain.Link, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var existingChatID int64
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM chats
		WHERE id = $1
	`, chatID).Scan(&existingChatID)
	if err != nil {
		return nil, usecase.ErrChatNotFound
	}

	var chatLinkID int64
	var linkID int64
	err = tx.QueryRow(ctx, `
		SELECT cl.id, l.id
		FROM chat_links cl
		JOIN links l ON l.id = cl.link_id
		WHERE cl.chat_id = $1 AND l.url = $2
	`, chatID, url).Scan(&chatLinkID, &linkID)
	if err != nil {
		return nil, usecase.ErrLinkNotFound
	}

	tags, err := r.getTagsByChatLinkIDTx(ctx, tx, chatLinkID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM chat_links
		WHERE id = $1
	`, chatLinkID)
	if err != nil {
		return nil, err
	}

	var cnt int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM chat_links
		WHERE link_id = $1
	`, linkID).Scan(&cnt)
	if err != nil {
		return nil, err
	}

	if cnt == 0 {
		_, err = tx.Exec(ctx, `
			DELETE FROM links
			WHERE id = $1
		`, linkID)
		if err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &domain.Link{
		ID:      chatLinkID,
		URL:     url,
		Tags:    tags,
		Filters: []string{},
	}, nil
}

func (r *LinkRepository) ListLinks(ctx context.Context) ([]*domain.Link, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, url
		FROM links
		ORDER BY url
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]*domain.Link, 0)
	for rows.Next() {
		var linkID int64
		var url string

		if err = rows.Scan(&linkID, &url); err != nil {
			return nil, err
		}

		res = append(res, &domain.Link{
			ID:  linkID,
			URL: url,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}

func (r *LinkRepository) GetChatIDsByLink(ctx context.Context, url string) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cl.chat_id
		FROM chat_links cl
		JOIN links l ON l.id = cl.link_id
		WHERE l.url = $1
		ORDER BY cl.chat_id
	`, url)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var chatID int64
		if err = rows.Scan(&chatID); err != nil {
			return nil, err
		}
		ids = append(ids, chatID)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, usecase.ErrLinkNotFound
	}

	return ids, nil
}

func (r *LinkRepository) GetURLState(ctx context.Context, url string) (domain.URLState, error) {
	var lastCheckedAt *time.Time
	var lastUpdatedAt *time.Time

	err := r.pool.QueryRow(ctx, `
		SELECT last_checked_at, last_updated_at
		FROM links
		WHERE url = $1
	`, url).Scan(&lastCheckedAt, &lastUpdatedAt)
	if err != nil {
		return domain.URLState{}, nil
	}

	var st domain.URLState
	if lastCheckedAt != nil {
		st.LastCheckedAt = *lastCheckedAt
	}
	if lastUpdatedAt != nil {
		st.LastUpdatedAt = *lastUpdatedAt
	}

	return st, nil
}

func (r *LinkRepository) SetURLState(ctx context.Context, url string, st domain.URLState) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE links
		SET last_checked_at = $2,
		    last_updated_at = $3,
		    updated_at = NOW()
		WHERE url = $1
	`, url, st.LastCheckedAt, st.LastUpdatedAt)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return usecase.ErrLinkNotFound
	}

	return nil
}

func (r *LinkRepository) getTagsByChatLinkID(ctx context.Context, chatLinkID int64) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.name
		FROM chat_link_tags clt
		JOIN tags t ON t.id = clt.tag_id
		WHERE clt.chat_link_id = $1
		ORDER BY t.name
	`, chatLinkID)
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

func (r *LinkRepository) getTagsByChatLinkIDTx(ctx context.Context, tx pgx.Tx, chatLinkID int64) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT t.name
		FROM chat_link_tags clt
		JOIN tags t ON t.id = clt.tag_id
		WHERE clt.chat_link_id = $1
		ORDER BY t.name
	`, chatLinkID)
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
