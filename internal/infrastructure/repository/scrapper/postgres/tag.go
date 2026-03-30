package postgres

import (
	"context"
	"errors"
	"fmt"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TagRepository struct {
	pool    *pgxpool.Pool
	dialect goqu.DialectWrapper
}

func NewTagRepository(pool *pgxpool.Pool) usecase.TagRepository {
	return &TagRepository{
		pool:    pool,
		dialect: goqu.Dialect("postgres"),
	}
}

func (r *TagRepository) CreateTag(ctx context.Context, chatID int64, name string) error {
	if err := r.ensureChatExists(ctx, chatID); err != nil {
		return err
	}

	sql, args, err := r.dialect.Insert("tags").
		Rows(goqu.Record{"name": name}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("build insert tag query: %w", err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return usecase.ErrTagAlreadyExist
		}
		return fmt.Errorf("exec insert tag: %w", err)
	}

	return nil
}

func (r *TagRepository) GetTagsByChatID(ctx context.Context, chatID int64) ([]string, error) {
	if err := r.ensureChatExists(ctx, chatID); err != nil {
		return nil, err
	}

	sql, args, err := r.dialect.From(goqu.T("tags").As("t")).
		Join(
			goqu.T("chat_link_tags").As("clt"),
			goqu.On(goqu.I("clt.tag_id").Eq(goqu.I("t.id"))),
		).
		Join(
			goqu.T("chat_links").As("cl"),
			goqu.On(goqu.I("cl.id").Eq(goqu.I("clt.chat_link_id"))),
		).
		Select(goqu.DISTINCT(goqu.I("t.name"))).
		Where(goqu.I("cl.chat_id").Eq(chatID)).
		Order(goqu.I("t.name").Asc()).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build select tags query: %w", err)
	}

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		scanErr := rows.Scan(&tag)
		if scanErr != nil {
			return nil, fmt.Errorf("scan tag: %w", scanErr)
		}
		tags = append(tags, tag)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("rows error: %w", rowsErr)
	}

	return tags, nil
}

func (r *TagRepository) UpdateTag(ctx context.Context, chatID int64, oldName, newName string) error {
	if err := r.ensureChatExists(ctx, chatID); err != nil {
		return err
	}

	oldTagID, err := r.getChatTagID(ctx, chatID, oldName)
	if err != nil {
		return err
	}

	newTagID, err := r.getOrCreateTagID(ctx, newName)
	if err != nil {
		return err
	}

	sql, args, err := r.dialect.Update(goqu.T("chat_link_tags").As("clt")).
		Set(goqu.Record{"tag_id": newTagID}).
		From(goqu.T("chat_links").As("cl")).
		Where(
			goqu.I("clt.chat_link_id").Eq(goqu.I("cl.id")),
			goqu.I("cl.chat_id").Eq(chatID),
			goqu.I("clt.tag_id").Eq(oldTagID),
		).
		ToSQL()
	if err != nil {
		return fmt.Errorf("build update tag query: %w", err)
	}

	_, execErr := r.pool.Exec(ctx, sql, args...)
	if execErr == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if !errors.As(execErr, &pgErr) || pgErr.Code != "23505" {
		return fmt.Errorf("exec update tag: %w", execErr)
	}

	sql, args, err = r.dialect.Delete("chat_link_tags").
		Where(
			goqu.C("chat_link_id").In(
				r.dialect.From(goqu.T("chat_links").As("cl")).
					Select(goqu.I("cl.id")).
					Where(goqu.I("cl.chat_id").Eq(chatID)),
			),
			goqu.C("tag_id").Eq(oldTagID),
		).
		ToSQL()
	if err != nil {
		return fmt.Errorf("build delete duplicate tag query: %w", err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("exec delete duplicate tag: %w", err)
	}

	return nil
}

func (r *TagRepository) DeleteTag(ctx context.Context, chatID int64, name string) error {
	if err := r.ensureChatExists(ctx, chatID); err != nil {
		return err
	}

	sql, args, err := r.dialect.Delete("chat_link_tags").
		Where(
			goqu.C("chat_link_id").In(
				r.dialect.From(goqu.T("chat_links").As("cl")).
					Select(goqu.I("cl.id")).
					Where(goqu.I("cl.chat_id").Eq(chatID)),
			),
			goqu.C("tag_id").In(
				r.dialect.From(goqu.T("tags").As("t")).
					Select(goqu.I("t.id")).
					Where(goqu.I("t.name").Eq(name)),
			),
		).
		ToSQL()
	if err != nil {
		return fmt.Errorf("build delete tag query: %w", err)
	}

	tag, err := r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("exec delete tag: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return usecase.ErrTagNotFound
	}

	return nil
}

func (r *TagRepository) ensureChatExists(ctx context.Context, chatID int64) error {
	sql, args, err := r.dialect.From("chats").
		Select("id").
		Where(goqu.C("id").Eq(chatID)).
		ToSQL()
	if err != nil {
		return fmt.Errorf("build ensure chat query: %w", err)
	}

	var id int64
	scanErr := r.pool.QueryRow(ctx, sql, args...).Scan(&id)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return usecase.ErrChatNotFound
		}
		return fmt.Errorf("scan chat: %w", scanErr)
	}

	return nil
}

func (r *TagRepository) getChatTagID(ctx context.Context, chatID int64, name string) (int64, error) {
	sql, args, err := r.dialect.From(goqu.T("tags").As("t")).
		Join(
			goqu.T("chat_link_tags").As("clt"),
			goqu.On(goqu.I("clt.tag_id").Eq(goqu.I("t.id"))),
		).
		Join(
			goqu.T("chat_links").As("cl"),
			goqu.On(goqu.I("cl.id").Eq(goqu.I("clt.chat_link_id"))),
		).
		Select(goqu.DISTINCT(goqu.I("t.id"))).
		Where(
			goqu.I("cl.chat_id").Eq(chatID),
			goqu.I("t.name").Eq(name),
		).
		ToSQL()
	if err != nil {
		return 0, fmt.Errorf("build get tag id query: %w", err)
	}

	var id int64
	scanErr := r.pool.QueryRow(ctx, sql, args...).Scan(&id)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return 0, usecase.ErrTagNotFound
		}
		return 0, fmt.Errorf("scan tag id: %w", scanErr)
	}

	return id, nil
}

func (r *TagRepository) getOrCreateTagID(ctx context.Context, name string) (int64, error) {
	sql, args, err := r.dialect.Insert("tags").
		Rows(goqu.Record{"name": name}).
		OnConflict(goqu.DoNothing()).
		ToSQL()
	if err != nil {
		return 0, fmt.Errorf("build insert tag query: %w", err)
	}

	_, execErr := r.pool.Exec(ctx, sql, args...)
	if execErr != nil {
		return 0, fmt.Errorf("exec insert tag: %w", execErr)
	}

	sql, args, err = r.dialect.From("tags").
		Select("id").
		Where(goqu.C("name").Eq(name)).
		ToSQL()
	if err != nil {
		return 0, fmt.Errorf("build select tag id query: %w", err)
	}

	var id int64
	scanErr := r.pool.QueryRow(ctx, sql, args...).Scan(&id)
	if scanErr != nil {
		return 0, fmt.Errorf("scan tag id: %w", scanErr)
	}

	return id, nil
}
