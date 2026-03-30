package rawpostgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	CreateLinkInsert = `
		INSERT INTO links (url)
		VALUES ($1)
		ON CONFLICT (url) DO NOTHING
	`
	CreateLinkSelect = `
		SELECT id
		FROM links
		WHERE url = $1
	`
	CreateLinkInsertChat = `
		INSERT INTO chat_links (chat_id, link_id)
		VALUES ($1, $2)
		RETURNING id
	`
	CreateLinkInsertTags = `
		INSERT INTO tags (name)
		VALUES ($1)
		ON CONFLICT (name) DO NOTHING
	`
	CreateLinkSelectTags = `
		SELECT id
		FROM tags
		WHERE name = $1
	`
	CreateLinkInsertChatLinkTags = `
		INSERT INTO chat_link_tags (chat_link_id, tag_id)
		VALUES ($1, $2)
		ON CONFLICT (chat_link_id, tag_id) DO NOTHING
	`

	GetLinksByChatIDSelectChat = `
		SELECT id
		FROM chats
		WHERE id = $1
	`
	GetLinksByChatIDSelectLinks = `
		SELECT cl.id, l.url
		FROM chat_links cl
		JOIN links l ON l.id = cl.link_id
		WHERE cl.chat_id = $1
		ORDER BY l.url
	`

	DeleteLinkSelectChat = `
		SELECT id
		FROM chats
		WHERE id = $1
	`
	DeleteLinkSelectChatLink = `
		SELECT cl.id, l.id
		FROM chat_links cl
		JOIN links l ON l.id = cl.link_id
		WHERE cl.chat_id = $1 AND l.url = $2
	`
	DeleteLinkDeleteChatLink = `
		DELETE FROM chat_links
		WHERE id = $1
	`
	DeleteLinkCountChatLinksByLinkID = `
		SELECT COUNT(*)
		FROM chat_links
		WHERE link_id = $1
	`
	DeleteLinkDeleteLink = `
		DELETE FROM links
		WHERE id = $1
	`

	ListLinksSelect = `
		SELECT id, url
		FROM links
		ORDER BY url
	`

	GetChatIDsByLinkSelect = `
		SELECT cl.chat_id
		FROM chat_links cl
		JOIN links l ON l.id = cl.link_id
		WHERE l.url = $1
		ORDER BY cl.chat_id
	`

	GetURLStateSelect = `
		SELECT last_checked_at, last_updated_at
		FROM links
		WHERE url = $1
	`

	SetURLStateUpdate = `
		UPDATE links
		SET last_checked_at = $2,
		    last_updated_at = $3,
		    updated_at = NOW()
		WHERE url = $1
	`

	GetTagsByChatLinkIDSelect = `
		SELECT t.name
		FROM chat_link_tags clt
		JOIN tags t ON t.id = clt.tag_id
		WHERE clt.chat_link_id = $1
		ORDER BY t.name
	`
)

type LinkRepository struct {
	pool *pgxpool.Pool
}

func NewLinkRepository(pool *pgxpool.Pool) usecase.LinkRepository {
	return &LinkRepository{pool: pool}
}

func rollbackTx(ctx context.Context, tx pgx.Tx) {
	rollbackErr := tx.Rollback(ctx)
	if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
		return
	}
}

func (r *LinkRepository) CreateLink(
	ctx context.Context,
	chatID int64,
	url string,
	tags, filters []string,
) (*domain.Link, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin transaction for create link: %w", err)
	}
	defer rollbackTx(ctx, tx)

	_, err = tx.Exec(ctx, CreateLinkInsert, url)
	if err != nil {
		return nil, fmt.Errorf("insert link url=%q: %w", url, err)
	}

	var linkID int64
	err = tx.QueryRow(ctx, CreateLinkSelect, url).Scan(&linkID)
	if err != nil {
		return nil, fmt.Errorf("select link id by url=%q: %w", url, err)
	}

	var chatLinkID int64
	err = tx.QueryRow(ctx, CreateLinkInsertChat, chatID, linkID).Scan(&chatLinkID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, usecase.ErrChatNotFound
		}
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, usecase.ErrLinkAlreadyTracked
		}
		return nil, fmt.Errorf("insert chat_link chat_id=%d link_id=%d: %w", chatID, linkID, err)
	}

	for _, tag := range tags {
		_, err = tx.Exec(ctx, CreateLinkInsertTags, tag)
		if err != nil {
			return nil, fmt.Errorf("insert tag name=%q: %w", tag, err)
		}

		var tagID int64
		err = tx.QueryRow(ctx, CreateLinkSelectTags, tag).Scan(&tagID)
		if err != nil {
			return nil, fmt.Errorf("select tag id by name=%q: %w", tag, err)
		}

		_, err = tx.Exec(ctx, CreateLinkInsertChatLinkTags, chatLinkID, tagID)
		if err != nil {
			return nil, fmt.Errorf(
				"insert chat_link_tag chat_link_id=%d tag_id=%d: %w",
				chatLinkID,
				tagID,
				err,
			)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("commit transaction for create link: %w", err)
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
	err := r.pool.QueryRow(ctx, GetLinksByChatIDSelectChat, chatID).Scan(&existingChatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []*domain.Link{}, usecase.ErrChatNotFound
		}
		return nil, fmt.Errorf("select chat id=%d: %w", chatID, err)
	}

	rows, err := r.pool.Query(ctx, GetLinksByChatIDSelectLinks, chatID)
	if err != nil {
		return nil, fmt.Errorf("query links by chat_id=%d: %w", chatID, err)
	}
	defer rows.Close()

	res := make([]*domain.Link, 0)
	for rows.Next() {
		var chatLinkID int64
		var url string

		scanErr := rows.Scan(&chatLinkID, &url)
		if scanErr != nil {
			return nil, fmt.Errorf("scan link row for chat_id=%d: %w", chatID, scanErr)
		}

		tags, tagsErr := r.getTagsByChatLinkID(ctx, chatLinkID)
		if tagsErr != nil {
			return nil, fmt.Errorf("get tags by chat_link_id=%d: %w", chatLinkID, tagsErr)
		}

		res = append(res, &domain.Link{
			ID:      chatLinkID,
			URL:     url,
			Tags:    tags,
			Filters: []string{},
		})
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("iterate links by chat_id=%d: %w", chatID, rowsErr)
	}

	return res, nil
}

func (r *LinkRepository) DeleteLink(ctx context.Context, chatID int64, url string) (*domain.Link, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin transaction for delete link: %w", err)
	}
	defer rollbackTx(ctx, tx)

	var existingChatID int64
	err = tx.QueryRow(ctx, DeleteLinkSelectChat, chatID).Scan(&existingChatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, usecase.ErrChatNotFound
		}
		return nil, fmt.Errorf("select chat id=%d: %w", chatID, err)
	}

	var chatLinkID int64
	var linkID int64
	err = tx.QueryRow(ctx, DeleteLinkSelectChatLink, chatID, url).Scan(&chatLinkID, &linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, usecase.ErrLinkNotFound
		}
		return nil, fmt.Errorf("select chat_link by chat_id=%d url=%q: %w", chatID, url, err)
	}

	tags, err := r.getTagsByChatLinkIDTx(ctx, tx, chatLinkID)
	if err != nil {
		return nil, fmt.Errorf("get tags by chat_link_id=%d in tx: %w", chatLinkID, err)
	}

	_, err = tx.Exec(ctx, DeleteLinkDeleteChatLink, chatLinkID)
	if err != nil {
		return nil, fmt.Errorf("delete chat_link id=%d: %w", chatLinkID, err)
	}

	var cnt int
	err = tx.QueryRow(ctx, DeleteLinkCountChatLinksByLinkID, linkID).Scan(&cnt)
	if err != nil {
		return nil, fmt.Errorf("count chat_links by link_id=%d: %w", linkID, err)
	}

	if cnt == 0 {
		_, err = tx.Exec(ctx, DeleteLinkDeleteLink, linkID)
		if err != nil {
			return nil, fmt.Errorf("delete link id=%d: %w", linkID, err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("commit transaction for delete link: %w", err)
	}

	return &domain.Link{
		ID:      chatLinkID,
		URL:     url,
		Tags:    tags,
		Filters: []string{},
	}, nil
}

func (r *LinkRepository) ListLinks(ctx context.Context) ([]*domain.Link, error) {
	rows, err := r.pool.Query(ctx, ListLinksSelect)
	if err != nil {
		return nil, fmt.Errorf("query list links: %w", err)
	}
	defer rows.Close()

	res := make([]*domain.Link, 0)
	for rows.Next() {
		var linkID int64
		var url string

		scanErr := rows.Scan(&linkID, &url)
		if scanErr != nil {
			return nil, fmt.Errorf("scan link row: %w", scanErr)
		}

		res = append(res, &domain.Link{
			ID:  linkID,
			URL: url,
		})
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("iterate list links rows: %w", rowsErr)
	}

	return res, nil
}

func (r *LinkRepository) GetChatIDsByLink(ctx context.Context, url string) ([]int64, error) {
	rows, err := r.pool.Query(ctx, GetChatIDsByLinkSelect, url)
	if err != nil {
		return nil, fmt.Errorf("query chat ids by url=%q: %w", url, err)
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var chatID int64

		scanErr := rows.Scan(&chatID)
		if scanErr != nil {
			return nil, fmt.Errorf("scan chat_id by url=%q: %w", url, scanErr)
		}

		ids = append(ids, chatID)
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("iterate chat ids by url=%q: %w", url, rowsErr)
	}

	if len(ids) == 0 {
		return nil, usecase.ErrLinkNotFound
	}

	return ids, nil
}

func (r *LinkRepository) GetURLState(ctx context.Context, url string) (domain.URLState, error) {
	var lastCheckedAt *time.Time
	var lastUpdatedAt *time.Time

	err := r.pool.QueryRow(ctx, GetURLStateSelect, url).Scan(&lastCheckedAt, &lastUpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.URLState{}, nil
		}
		return domain.URLState{}, fmt.Errorf("select url state by url=%q: %w", url, err)
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
	tag, err := r.pool.Exec(ctx, SetURLStateUpdate, url, st.LastCheckedAt, st.LastUpdatedAt)
	if err != nil {
		return fmt.Errorf("update url state for url=%q: %w", url, err)
	}

	if tag.RowsAffected() == 0 {
		return usecase.ErrLinkNotFound
	}

	return nil
}

func (r *LinkRepository) getTagsByChatLinkID(ctx context.Context, chatLinkID int64) ([]string, error) {
	rows, err := r.pool.Query(ctx, GetTagsByChatLinkIDSelect, chatLinkID)
	if err != nil {
		return nil, fmt.Errorf("query tags by chat_link_id=%d: %w", chatLinkID, err)
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var tag string

		scanErr := rows.Scan(&tag)
		if scanErr != nil {
			return nil, fmt.Errorf("scan tag for chat_link_id=%d: %w", chatLinkID, scanErr)
		}

		tags = append(tags, tag)
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("iterate tags by chat_link_id=%d: %w", chatLinkID, rowsErr)
	}

	return tags, nil
}

func (r *LinkRepository) getTagsByChatLinkIDTx(ctx context.Context, tx pgx.Tx, chatLinkID int64) ([]string, error) {
	rows, err := tx.Query(ctx, GetTagsByChatLinkIDSelect, chatLinkID)
	if err != nil {
		return nil, fmt.Errorf("query tags by chat_link_id=%d in tx: %w", chatLinkID, err)
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var tag string

		scanErr := rows.Scan(&tag)
		if scanErr != nil {
			return nil, fmt.Errorf("scan tag for chat_link_id=%d in tx: %w", chatLinkID, scanErr)
		}

		tags = append(tags, tag)
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("iterate tags by chat_link_id=%d in tx: %w", chatLinkID, rowsErr)
	}

	return tags, nil
}
