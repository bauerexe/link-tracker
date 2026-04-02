package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
)

type GithubRepository struct {
	pool    *pgxpool.Pool
	dialect goqu.DialectWrapper
}

func NewGithubRepository(pool *pgxpool.Pool) github.Repository {
	return &GithubRepository{
		pool:    pool,
		dialect: goqu.Dialect("postgres"),
	}
}

func (g GithubRepository) CreateState(ctx context.Context, repoFullName string, state github.State) error {
	sql, _, err := g.dialect.Insert("github_sync_state").
		Rows(goqu.Record{
			"repo_full_name":              repoFullName,
			"last_repo_updated_at":        state.LastRepoUpdated,
			"last_processed_issue_number": state.LastProcessesIssueNumber,
		}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("build create github state query: %w", err)
	}

	_, err = g.pool.Exec(ctx, sql)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return github.ErrStateAlreadyExists
		}
		return fmt.Errorf("exec create github state: %w", err)
	}

	return nil
}

func (g GithubRepository) GetState(ctx context.Context, repoFullName string) (st *github.State, e error) {
	sql, _, err := g.dialect.From("github_syncs_state").
		Select(&st).
		Where(goqu.Ex{"repo_full_name": repoFullName}).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build get github state query: %w", err)
	}
	_, err = g.pool.Exec(ctx, sql)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, github.ErrStateAlreadyExists
		}
		return nil, fmt.Errorf("exec get github state: %w", err)
	}
	return st, nil
}

func (g GithubRepository) UpdateState(ctx context.Context, repoFullName string, state github.State) error {
	sql, _, err := g.dialect.From("github_sync_state").
		Update().Set(goqu.Record{
		"last_repo_updated_at":        state.LastRepoUpdated,
		"last_processed_issue_number": state.LastProcessesIssueNumber,
	}).Where(goqu.Ex{"repo_full_name": repoFullName}).ToSQL()

	if err != nil {
		return fmt.Errorf("build update github state query: %w", err)
	}

	_, err = g.pool.Exec(ctx, sql)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return github.ErrStateAlreadyExists
		}
		return fmt.Errorf("exec update github state query: %w", err)
	}
	return nil
}
