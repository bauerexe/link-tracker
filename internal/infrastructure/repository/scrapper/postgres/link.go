package postgres

import (
	"context"
	"errors"
	"time"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LinkRepository struct {
	pool    *pgxpool.Pool
	dialect goqu.DialectWrapper
}

type db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func NewLinkRepository(pool *pgxpool.Pool) usecase.LinkRepository {
	return &LinkRepository{
		pool:    pool,
		dialect: goqu.Dialect("postgres"),
	}
}

func (r *LinkRepository) CreateLink(ctx context.Context, chatID int64, url string, tags, filters []string) (*domain.Link, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	linkID, err := r.getOrCreateLinkID(ctx, tx, url)
	if err != nil {
		return nil, err
	}

	chatLinkID, err := r.insertChatLink(ctx, tx, chatID, linkID)
	if err != nil {
		return nil, err
	}

	for _, tag := range tags {
		tagID, err := r.getOrCreateTagID(ctx, tx, tag)
		if err != nil {
			return nil, err
		}
		if err = r.insertChatLinkTag(ctx, tx, chatLinkID, tagID); err != nil {
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
	if err := r.ensureChatExists(ctx, r.pool, chatID); err != nil {
		return nil, err
	}

	sql, args, err := r.dialect.From(goqu.T("chat_links").As("cl")).
		Join(
			goqu.T("links").As("l"),
			goqu.On(goqu.I("l.id").Eq(goqu.I("cl.link_id"))),
		).
		Select(
			goqu.I("cl.id"),
			goqu.I("l.url"),
		).
		Where(goqu.I("cl.chat_id").Eq(chatID)).
		Order(goqu.I("l.url").Asc()).
		ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*domain.Link
	for rows.Next() {
		var id int64
		var url string

		if err = rows.Scan(&id, &url); err != nil {
			return nil, err
		}

		tags, err := r.getTagsByChatLinkID(ctx, r.pool, id)
		if err != nil {
			return nil, err
		}

		links = append(links, &domain.Link{
			ID:      id,
			URL:     url,
			Tags:    tags,
			Filters: []string{},
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return links, nil
}

func (r *LinkRepository) DeleteLink(ctx context.Context, chatID int64, url string) (*domain.Link, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err = r.ensureChatExists(ctx, tx, chatID); err != nil {
		return nil, err
	}

	chatLinkID, linkID, err := r.findChatLink(ctx, tx, chatID, url)
	if err != nil {
		return nil, err
	}

	tags, err := r.getTagsByChatLinkID(ctx, tx, chatLinkID)
	if err != nil {
		return nil, err
	}

	sql, args, err := r.dialect.Delete("chat_links").
		Where(goqu.C("id").Eq(chatLinkID)).
		ToSQL()
	if err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx, sql, args...); err != nil {
		return nil, err
	}

	sql, args, err = r.dialect.From("chat_links").
		Select(goqu.COUNT("*")).
		Where(goqu.C("link_id").Eq(linkID)).
		ToSQL()
	if err != nil {
		return nil, err
	}

	var cnt int
	if err = tx.QueryRow(ctx, sql, args...).Scan(&cnt); err != nil {
		return nil, err
	}

	if cnt == 0 {
		sql, args, err = r.dialect.Delete("links").
			Where(goqu.C("id").Eq(linkID)).
			ToSQL()
		if err != nil {
			return nil, err
		}

		if _, err = tx.Exec(ctx, sql, args...); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
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
	sql, args, err := r.dialect.From("links").
		Select("id", "url").
		Order(goqu.C("url").Asc()).
		ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*domain.Link
	for rows.Next() {
		var id int64
		var url string

		if err = rows.Scan(&id, &url); err != nil {
			return nil, err
		}

		links = append(links, &domain.Link{
			ID:  id,
			URL: url,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return links, nil
}

func (r *LinkRepository) GetChatIDsByLink(ctx context.Context, url string) ([]int64, error) {
	sql, args, err := r.dialect.From(goqu.T("chat_links").As("cl")).
		Join(
			goqu.T("links").As("l"),
			goqu.On(goqu.I("l.id").Eq(goqu.I("cl.link_id"))),
		).
		Select(goqu.I("cl.chat_id")).
		Where(goqu.I("l.url").Eq(url)).
		Order(goqu.I("cl.chat_id").Asc()).
		ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
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
	sql, args, err := r.dialect.From("links").
		Select("last_checked_at", "last_updated_at").
		Where(goqu.C("url").Eq(url)).
		ToSQL()
	if err != nil {
		return domain.URLState{}, err
	}

	var lastCheckedAt *time.Time
	var lastUpdatedAt *time.Time

	if err = r.pool.QueryRow(ctx, sql, args...).Scan(&lastCheckedAt, &lastUpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.URLState{}, nil
		}
		return domain.URLState{}, err
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
	sql, args, err := r.dialect.Update("links").
		Set(goqu.Record{
			"last_checked_at": st.LastCheckedAt,
			"last_updated_at": st.LastUpdatedAt,
			"updated_at":      goqu.L("NOW()"),
		}).
		Where(goqu.C("url").Eq(url)).
		ToSQL()
	if err != nil {
		return err
	}

	tag, err := r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return usecase.ErrLinkNotFound
	}

	return nil
}

func (r *LinkRepository) ensureChatExists(ctx context.Context, q db, chatID int64) error {
	sql, args, err := r.dialect.From("chats").
		Select("id").
		Where(goqu.C("id").Eq(chatID)).
		ToSQL()
	if err != nil {
		return err
	}

	var id int64
	if err = q.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return usecase.ErrChatNotFound
		}
		return err
	}

	return nil
}

func (r *LinkRepository) getOrCreateLinkID(ctx context.Context, q db, url string) (int64, error) {
	sql, args, err := r.dialect.Insert("links").
		Rows(goqu.Record{"url": url}).
		OnConflict(goqu.DoNothing()).
		ToSQL()
	if err != nil {
		return 0, err
	}

	if _, err = q.Exec(ctx, sql, args...); err != nil {
		return 0, err
	}

	sql, args, err = r.dialect.From("links").
		Select("id").
		Where(goqu.C("url").Eq(url)).
		ToSQL()
	if err != nil {
		return 0, err
	}

	var id int64
	if err = q.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		return 0, err
	}

	return id, nil
}

func (r *LinkRepository) getOrCreateTagID(ctx context.Context, q db, name string) (int64, error) {
	sql, args, err := r.dialect.Insert("tags").
		Rows(goqu.Record{"name": name}).
		OnConflict(goqu.DoNothing()).
		ToSQL()
	if err != nil {
		return 0, err
	}

	if _, err = q.Exec(ctx, sql, args...); err != nil {
		return 0, err
	}

	sql, args, err = r.dialect.From("tags").
		Select("id").
		Where(goqu.C("name").Eq(name)).
		ToSQL()
	if err != nil {
		return 0, err
	}

	var id int64
	if err = q.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		return 0, err
	}

	return id, nil
}

func (r *LinkRepository) insertChatLink(ctx context.Context, q db, chatID, linkID int64) (int64, error) {
	sql, args, err := r.dialect.Insert("chat_links").
		Rows(goqu.Record{
			"chat_id": chatID,
			"link_id": linkID,
		}).
		Returning("id").
		ToSQL()
	if err != nil {
		return 0, err
	}

	var id int64
	if err := q.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503":
				return 0, usecase.ErrChatNotFound
			case "23505":
				return 0, usecase.ErrLinkAlreadyTracked
			}
		}
		return 0, err
	}

	return id, nil
}

func (r *LinkRepository) insertChatLinkTag(ctx context.Context, q db, chatLinkID, tagID int64) error {
	sql, args, err := r.dialect.Insert("chat_link_tags").
		Rows(goqu.Record{
			"chat_link_id": chatLinkID,
			"tag_id":       tagID,
		}).
		OnConflict(goqu.DoNothing()).
		ToSQL()
	if err != nil {
		return err
	}

	_, err = q.Exec(ctx, sql, args...)
	return err
}

func (r *LinkRepository) findChatLink(ctx context.Context, q db, chatID int64, url string) (int64, int64, error) {
	sql, args, err := r.dialect.From(goqu.T("chat_links").As("cl")).
		Join(
			goqu.T("links").As("l"),
			goqu.On(goqu.I("l.id").Eq(goqu.I("cl.link_id"))),
		).
		Select(goqu.I("cl.id"), goqu.I("l.id")).
		Where(
			goqu.I("cl.chat_id").Eq(chatID),
			goqu.I("l.url").Eq(url),
		).
		ToSQL()
	if err != nil {
		return 0, 0, err
	}

	var chatLinkID, linkID int64
	if err = q.QueryRow(ctx, sql, args...).Scan(&chatLinkID, &linkID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, usecase.ErrLinkNotFound
		}
		return 0, 0, err
	}

	return chatLinkID, linkID, nil
}

func (r *LinkRepository) getTagsByChatLinkID(ctx context.Context, q db, chatLinkID int64) ([]string, error) {
	sql, args, err := r.dialect.From(goqu.T("chat_link_tags").As("clt")).
		Join(
			goqu.T("tags").As("t"),
			goqu.On(goqu.I("t.id").Eq(goqu.I("clt.tag_id"))),
		).
		Select(goqu.I("t.name")).
		Where(goqu.I("clt.chat_link_id").Eq(chatLinkID)).
		Order(goqu.I("t.name").Asc()).
		ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
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
