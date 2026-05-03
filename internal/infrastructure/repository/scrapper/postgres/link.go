package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	dbtx "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/db"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type LinkRepository struct {
	pool    *pgxpool.Pool
	dialect goqu.DialectWrapper
}

func (r *LinkRepository) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := dbtx.InTx(ctx, r.pool, fn); err != nil {
		return fmt.Errorf("error in link repository: %w", err)
	}

	return nil
}

type db = dbtx.Executor

func NewLinkRepository(pool *pgxpool.Pool) usecase.LinkRepository {
	return &LinkRepository{
		pool:    pool,
		dialect: goqu.Dialect("postgres"),
	}
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

	linkID, err := r.getOrCreateLinkID(ctx, tx, url)
	if err != nil {
		return nil, fmt.Errorf("get or create link id for %q: %w", url, err)
	}

	chatLinkID, err := r.insertChatLink(ctx, tx, chatID, linkID)
	if err != nil {
		return nil, fmt.Errorf("insert chat link for chat_id=%d link_id=%d: %w", chatID, linkID, err)
	}

	for _, tag := range tags {
		tagID, getTagErr := r.getOrCreateTagID(ctx, tx, tag)
		if getTagErr != nil {
			return nil, fmt.Errorf("get or create tag id for %q: %w", tag, getTagErr)
		}

		insertTagErr := r.insertChatLinkTag(ctx, tx, chatLinkID, tagID)
		if insertTagErr != nil {
			return nil, fmt.Errorf(
				"insert chat link tag for chat_link_id=%d tag_id=%d: %w",
				chatLinkID,
				tagID,
				insertTagErr,
			)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction for create link: %w", err)
	}

	return &domain.Link{
		ID:      chatLinkID,
		URL:     url,
		Tags:    append([]string(nil), tags...),
		Filters: append([]string(nil), filters...),
	}, nil
}

func (r *LinkRepository) GetLinksByChatID(
	ctx context.Context,
	chatID int64,
	limit, offset uint64,
) ([]*domain.Link, error) {
	err := r.ensureChatExists(ctx, r.pool, chatID)
	if err != nil {
		return nil, fmt.Errorf("ensure chat exists for chat_id=%d: %w", chatID, err)
	}

	ds := r.dialect.
		From(goqu.T("chat_links").As("cl")).
		Join(
			goqu.T("links").As("l"),
			goqu.On(goqu.I("l.id").Eq(goqu.I("cl.link_id"))),
		).
		LeftJoin(
			goqu.T("chat_link_tags").As("clt"),
			goqu.On(goqu.I("clt.chat_link_id").Eq(goqu.I("cl.id"))),
		).
		LeftJoin(
			goqu.T("tags").As("t"),
			goqu.On(goqu.I("t.id").Eq(goqu.I("clt.tag_id"))),
		).
		Select(
			goqu.I("cl.id"),
			goqu.I("l.url"),
			goqu.L(
				"COALESCE(array_agg(t.name ORDER BY t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tags",
			),
		).
		Where(goqu.I("cl.chat_id").Eq(chatID)).
		GroupBy(goqu.I("cl.id"), goqu.I("l.url")).
		Order(goqu.I("cl.id").Asc()).
		Limit(uint(limit)).
		Offset(uint(offset))

	sql, args, err := ds.ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build query for links by chat_id=%d: %w", chatID, err)
	}

	rows, err := r.executor(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query links by chat_id=%d: %w", chatID, err)
	}
	defer rows.Close()

	var links []*domain.Link
	for rows.Next() {
		var id int64
		var url string
		var tags []string

		if err = rows.Scan(&id, &url, &tags); err != nil {
			return nil, fmt.Errorf("scan link row for chat_id=%d: %w", chatID, err)
		}

		links = append(links, &domain.Link{
			ID:      id,
			URL:     url,
			Tags:    tags,
			Filters: []string{},
		})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate links by chat_id=%d: %w", chatID, err)
	}

	return links, nil
}

func (r *LinkRepository) DeleteLink(ctx context.Context, chatID int64, url string) (*domain.Link, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin transaction for delete link: %w", err)
	}
	defer rollbackTx(ctx, tx)

	err = r.ensureChatExists(ctx, tx, chatID)
	if err != nil {
		return nil, fmt.Errorf("ensure chat exists for chat_id=%d: %w", chatID, err)
	}

	chatLinkID, linkID, err := r.findChatLink(ctx, tx, chatID, url)
	if err != nil {
		return nil, fmt.Errorf("find chat link for chat_id=%d url=%q: %w", chatID, url, err)
	}

	tags, err := r.getTagsByChatLinkID(ctx, tx, chatLinkID)
	if err != nil {
		return nil, fmt.Errorf("get tags by chat_link_id=%d: %w", chatLinkID, err)
	}

	sql, args, err := r.dialect.Delete("chat_links").
		Where(goqu.C("id").Eq(chatLinkID)).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build delete chat_link query for id=%d: %w", chatLinkID, err)
	}

	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("delete chat_link id=%d: %w", chatLinkID, err)
	}

	sql, args, err = r.dialect.From("chat_links").
		Select(goqu.COUNT("*")).
		Where(goqu.C("link_id").Eq(linkID)).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build count chat_links for link_id=%d: %w", linkID, err)
	}

	var cnt int
	err = tx.QueryRow(ctx, sql, args...).Scan(&cnt)
	if err != nil {
		return nil, fmt.Errorf("count chat_links for link_id=%d: %w", linkID, err)
	}

	if cnt == 0 {
		sql, args, err = r.dialect.Delete("links").
			Where(goqu.C("id").Eq(linkID)).
			ToSQL()
		if err != nil {
			return nil, fmt.Errorf("build delete link query for id=%d: %w", linkID, err)
		}

		_, err = tx.Exec(ctx, sql, args...)
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

func (r *LinkRepository) ListLinksBatch(ctx context.Context, limit, offset int) ([]*domain.Link, error) {
	sql, args, err := r.dialect.From("links").
		Select("id", "url").
		Order(goqu.C("url").Asc()).
		Limit(uint(limit)).
		Offset(uint(offset)).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build query for list links batch: %w", err)
	}

	rows, err := r.executor(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query list links batch: %w", err)
	}
	defer rows.Close()

	links := make([]*domain.Link, 0, limit)

	for rows.Next() {
		var id int64
		var url string

		scanErr := rows.Scan(&id, &url)
		if scanErr != nil {
			return nil, fmt.Errorf("scan link row: %w", scanErr)
		}

		links = append(links, &domain.Link{
			ID:  id,
			URL: url,
		})
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate list links batch rows: %w", rowsErr)
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
		return nil, fmt.Errorf("build query for chat ids by url=%q: %w", url, err)
	}

	rows, err := r.executor(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query chat ids by url=%q: %w", url, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64

		scanErr := rows.Scan(&id)
		if scanErr != nil {
			return nil, fmt.Errorf("scan chat_id for url=%q: %w", url, scanErr)
		}

		ids = append(ids, id)
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("iterate chat ids for url=%q: %w", url, rowsErr)
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
		return domain.URLState{}, fmt.Errorf("build query for url state url=%q: %w", url, err)
	}

	var lastCheckedAt *time.Time
	var lastUpdatedAt *time.Time

	err = r.pool.QueryRow(ctx, sql, args...).Scan(&lastCheckedAt, &lastUpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.URLState{}, nil
		}

		return domain.URLState{}, fmt.Errorf("query url state url=%q: %w", url, err)
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
		return fmt.Errorf("build update url state query for url=%q: %w", url, err)
	}

	tag, err := r.executor(ctx).Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("update url state for url=%q: %w", url, err)
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
		return fmt.Errorf("build query for ensure chat exists chat_id=%d: %w", chatID, err)
	}

	var id int64
	err = q.QueryRow(ctx, sql, args...).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return usecase.ErrChatNotFound
		}

		return fmt.Errorf("query ensure chat exists chat_id=%d: %w", chatID, err)
	}

	return nil
}

func (r *LinkRepository) getOrCreateLinkID(ctx context.Context, q db, url string) (int64, error) {
	return r.getOrCreateID(
		ctx,
		q,
		"links",
		"url",
		url,
	)
}

func (r *LinkRepository) getOrCreateTagID(ctx context.Context, q db, name string) (int64, error) {
	return r.getOrCreateID(
		ctx,
		q,
		"tags",
		"name",
		name,
	)
}

func (r *LinkRepository) getOrCreateID(
	ctx context.Context,
	q db,
	table string,
	uniqueColumn string,
	value string,
) (int64, error) {
	sql, args, err := r.dialect.Insert(table).
		Rows(goqu.Record{uniqueColumn: value}).
		OnConflict(goqu.DoNothing()).
		ToSQL()
	if err != nil {
		return 0, fmt.Errorf("build insert query for %s.%s=%q: %w", table, uniqueColumn, value, err)
	}

	_, err = q.Exec(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("exec insert for %s.%s=%q: %w", table, uniqueColumn, value, err)
	}

	sql, args, err = r.dialect.From(table).
		Select("id").
		Where(goqu.C(uniqueColumn).Eq(value)).
		ToSQL()
	if err != nil {
		return 0, fmt.Errorf("build select id query for %s.%s=%q: %w", table, uniqueColumn, value, err)
	}

	var id int64
	err = q.QueryRow(ctx, sql, args...).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("select id for %s.%s=%q: %w", table, uniqueColumn, value, err)
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
		return 0, fmt.Errorf("build insert chat_link query chat_id=%d link_id=%d: %w", chatID, linkID, err)
	}

	var id int64
	err = q.QueryRow(ctx, sql, args...).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503":
				return 0, usecase.ErrChatNotFound
			case "23505":
				return 0, usecase.ErrLinkAlreadyTracked
			}
		}

		return 0, fmt.Errorf("insert chat_link chat_id=%d link_id=%d: %w", chatID, linkID, err)
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
		return fmt.Errorf(
			"build insert chat_link_tag query chat_link_id=%d tag_id=%d: %w",
			chatLinkID,
			tagID,
			err,
		)
	}

	_, err = q.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("insert chat_link_tag chat_link_id=%d tag_id=%d: %w", chatLinkID, tagID, err)
	}

	return nil
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
		return 0, 0, fmt.Errorf("build query for find chat link chat_id=%d url=%q: %w", chatID, url, err)
	}

	var chatLinkID int64
	var linkID int64

	err = q.QueryRow(ctx, sql, args...).Scan(&chatLinkID, &linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, usecase.ErrLinkNotFound
		}

		return 0, 0, fmt.Errorf("query find chat link chat_id=%d url=%q: %w", chatID, url, err)
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
		return nil, fmt.Errorf("build query for tags by chat_link_id=%d: %w", chatLinkID, err)
	}

	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query tags by chat_link_id=%d: %w", chatLinkID, err)
	}
	defer rows.Close()

	var tags []string
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
		return nil, fmt.Errorf("iterate tags for chat_link_id=%d: %w", chatLinkID, rowsErr)
	}

	return tags, nil
}

func (r *LinkRepository) executor(ctx context.Context) dbtx.Executor {
	return dbtx.ExecutorFromContext(ctx, r.pool)
}
