package integration

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	ormrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/scrapper/postgres"
	rawrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/repository/scrapper/rawpostgres"
)

type dbEnv struct {
	pool *pgxpool.Pool
	tc   testcontainers.Container
}

type repos struct {
	chat usecase.ChatRepository
	link usecase.LinkRepository
	tag  usecase.TagRepository
}

func TestDBRepositories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("testcontainers rootless Docker is not supported on Windows")
	}
	if os.Getenv("CI") != "" && os.Getenv("DOCKER_HOST") == "" {
		t.Skip("docker is not available in this job")
	}
	cases := []struct {
		name string
		make func(*pgxpool.Pool) repos
	}{
		{
			name: "orm",
			make: func(pool *pgxpool.Pool) repos {
				return repos{
					chat: ormrepo.NewChatRepository(pool),
					link: ormrepo.NewLinkRepository(pool),
					tag:  ormrepo.NewTagRepository(pool),
				}
			},
		},
		{
			name: "raw_sql",
			make: func(pool *pgxpool.Pool) repos {
				return repos{
					chat: rawrepo.NewChatRepository(pool),
					link: rawrepo.NewLinkRepository(pool),
					tag:  rawrepo.NewTagRepository(pool),
				}
			},
		},
	}

	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {
			env := mustStartPostgres(t)
			defer env.Close(t)

			r := tc.make(env.pool)
			ctx := context.Background()

			t.Run("chat_crud", func(t *testing.T) {
				mustResetDB(ctx, t, env.pool)

				created, err := r.chat.CreateChat(ctx, 1)
				require.NoError(t, err)
				require.Equal(t, int64(1), created.ID)

				got, err := r.chat.GetChatByID(ctx, 1)
				require.NoError(t, err)
				require.Equal(t, int64(1), got.ID)

				deleted, err := r.chat.DeleteChatByID(ctx, 1)
				require.NoError(t, err)
				require.Equal(t, int64(1), deleted.ID)
			})

			t.Run("link_crud", func(t *testing.T) {
				mustResetDB(ctx, t, env.pool)

				_, err := r.chat.CreateChat(ctx, 1)
				require.NoError(t, err)

				created, err := r.link.CreateLink(ctx, 1, "https://example.com", []string{"go", "db"}, []string{"f1"})
				require.NoError(t, err)
				require.Equal(t, "https://example.com", created.URL)
				require.ElementsMatch(t, []string{"go", "db"}, created.Tags)

				links, err := r.link.GetLinksByChatID(ctx, 1)
				require.NoError(t, err)
				require.Len(t, links, 1)
				require.Equal(t, "https://example.com", links[0].URL)
				require.ElementsMatch(t, []string{"go", "db"}, links[0].Tags)

				all, err := r.link.ListLinksBatch(ctx, 1000, 0)
				require.NoError(t, err)
				require.Len(t, all, 1)
				require.Equal(t, "https://example.com", all[0].URL)

				ids, err := r.link.GetChatIDsByLink(ctx, "https://example.com")
				require.NoError(t, err)
				require.Equal(t, []int64{1}, ids)

				deleted, err := r.link.DeleteLink(ctx, 1, "https://example.com")
				require.NoError(t, err)
				require.Equal(t, "https://example.com", deleted.URL)

				links, err = r.link.GetLinksByChatID(ctx, 1)
				require.NoError(t, err)
				require.Empty(t, links)
			})

			t.Run("link_errors", func(t *testing.T) {
				mustResetDB(ctx, t, env.pool)

				_, err := r.link.CreateLink(ctx, 404, "https://example.com", nil, nil)
				require.ErrorIs(t, err, usecase.ErrChatNotFound)

				_, err = r.chat.CreateChat(ctx, 1)
				require.NoError(t, err)

				_, err = r.link.CreateLink(ctx, 1, "https://example.com", nil, nil)
				require.NoError(t, err)

				_, err = r.link.CreateLink(ctx, 1, "https://example.com", nil, nil)
				require.ErrorIs(t, err, usecase.ErrLinkAlreadyTracked)

				_, err = r.link.DeleteLink(ctx, 1, "https://missing.com")
				require.ErrorIs(t, err, usecase.ErrLinkNotFound)

				_, err = r.link.GetChatIDsByLink(ctx, "https://missing.com")
				require.ErrorIs(t, err, usecase.ErrLinkNotFound)
			})

			t.Run("url_state", func(t *testing.T) {
				mustResetDB(ctx, t, env.pool)

				_, err := r.chat.CreateChat(ctx, 1)
				require.NoError(t, err)

				_, err = r.link.CreateLink(ctx, 1, "https://example.com", nil, nil)
				require.NoError(t, err)

				got, err := r.link.GetURLState(ctx, "https://example.com")
				require.NoError(t, err)
				require.True(t, got.LastCheckedAt.IsZero())
				require.True(t, got.LastUpdatedAt.IsZero())

				st := domain.URLState{
					LastCheckedAt: time.Now().UTC().Truncate(time.Second),
					LastUpdatedAt: time.Now().UTC().Add(time.Minute).Truncate(time.Second),
				}

				err = r.link.SetURLState(ctx, "https://example.com", st)
				require.NoError(t, err)

				got, err = r.link.GetURLState(ctx, "https://example.com")
				require.NoError(t, err)
				require.WithinDuration(t, st.LastCheckedAt, got.LastCheckedAt, time.Second)
				require.WithinDuration(t, st.LastUpdatedAt, got.LastUpdatedAt, time.Second)

				err = r.link.SetURLState(ctx, "https://missing.com", st)
				require.ErrorIs(t, err, usecase.ErrLinkNotFound)
			})

			t.Run("tag_crud", func(t *testing.T) {
				mustResetDB(ctx, t, env.pool)

				_, err := r.chat.CreateChat(ctx, 1)
				require.NoError(t, err)

				err = r.tag.CreateTag(ctx, 1, "backend")
				require.NoError(t, err)

				_, err = r.link.CreateLink(ctx, 1, "https://example.com", []string{"backend"}, nil)
				require.NoError(t, err)

				tags, err := r.tag.GetTagsByChatID(ctx, 1)
				require.NoError(t, err)
				require.Contains(t, tags, "backend")

				err = r.tag.UpdateTag(ctx, 1, "backend", "golang")
				require.NoError(t, err)

				tags, err = r.tag.GetTagsByChatID(ctx, 1)
				require.NoError(t, err)
				require.Contains(t, tags, "golang")
				require.NotContains(t, tags, "backend")

				err = r.tag.DeleteTag(ctx, 1, "golang")
				require.NoError(t, err)

				tags, err = r.tag.GetTagsByChatID(ctx, 1)
				require.NoError(t, err)
				require.NotContains(t, tags, "golang")
			})

			t.Run("tag_errors", func(t *testing.T) {
				mustResetDB(ctx, t, env.pool)

				err := r.tag.CreateTag(ctx, 404, "backend")
				require.ErrorIs(t, err, usecase.ErrChatNotFound)

				_, err = r.chat.CreateChat(ctx, 1)
				require.NoError(t, err)

				err = r.tag.CreateTag(ctx, 1, "backend")
				require.NoError(t, err)

				err = r.tag.CreateTag(ctx, 1, "backend")
				require.ErrorIs(t, err, usecase.ErrTagAlreadyExist)

				err = r.tag.UpdateTag(ctx, 1, "missing", "new")
				require.ErrorIs(t, err, usecase.ErrTagNotFound)

				err = r.tag.DeleteTag(ctx, 1, "missing")
				require.ErrorIs(t, err, usecase.ErrTagNotFound)
			})
		})
	}
}

func mustStartPostgres(t *testing.T) *dbEnv {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * time.Second),
	}

	tc, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := tc.Host(ctx)
	require.NoError(t, err)

	port, err := tc.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	dsn := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return pool.Ping(ctx) == nil
	}, 10*time.Second, 200*time.Millisecond)

	mustApplySchema(ctx, t, pool)

	return &dbEnv{pool: pool, tc: tc}
}

func (e *dbEnv) Close(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	if e.pool != nil {
		e.pool.Close()
	}
	if e.tc != nil {
		_ = e.tc.Terminate(ctx)
	}
}

func mustApplySchema(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	schema := `
CREATE TABLE chats (
    id BIGINT PRIMARY KEY
);

CREATE TABLE links (
    id BIGSERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    last_checked_at TIMESTAMPTZ NULL,
    last_updated_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE tags (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE chat_links (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    link_id BIGINT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    UNIQUE (chat_id, link_id)
);

CREATE TABLE chat_link_tags (
    chat_link_id BIGINT NOT NULL REFERENCES chat_links(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (chat_link_id, tag_id)
);
`
	_, err := pool.Exec(ctx, schema)
	require.NoError(t, err)
}

func mustResetDB(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	_, err := pool.Exec(ctx, `
		TRUNCATE TABLE
			chat_link_tags,
			chat_links,
			tags,
			links,
			chats
		RESTART IDENTITY CASCADE
	`)
	require.NoError(t, err)
}
