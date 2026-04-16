package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
)

func TestLinkRepository_CreateLink(t *testing.T) {
	type testCase struct {
		name    string
		chatID  int64
		url     string
		tags    []string
		filters []string
		ctx     func() context.Context
		prepare func(rp *LinkRepository)
		err     error
	}

	testCases := []testCase{
		{
			name:    "positive 1",
			chatID:  1,
			url:     "http://example.com",
			tags:    []string{"t1"},
			filters: []string{"f1"},
			ctx:     context.Background,
			err:     nil,
		},
		{
			name:    "negative 1 already tracked",
			chatID:  1,
			url:     "http://example.com",
			tags:    []string{"t1"},
			filters: []string{"f1"},
			ctx:     context.Background,
			prepare: func(rp *LinkRepository) {
				_, _ = rp.CreateLink(context.Background(), 1, "http://example.com", []string{"t1"}, []string{"f1"})
			},
			err: usecase.ErrLinkAlreadyTracked,
		},
		{
			name:    "positive 2 another link same chat",
			chatID:  1,
			url:     "http://example.org",
			tags:    []string{"t2"},
			filters: []string{"f2"},
			ctx:     context.Background,
			prepare: func(rp *LinkRepository) {
				_, _ = rp.CreateLink(context.Background(), 1, "http://example.com", []string{"t1"}, []string{"f1"})
			},
			err: nil,
		},
		{
			name:    "negative 2 ctx is Done",
			chatID:  2,
			url:     "http://example.net",
			tags:    []string{"t3"},
			filters: []string{"f3"},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			err: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp := NewLinkRepository().(*LinkRepository)

			if tc.prepare != nil {
				tc.prepare(rp)
			}

			link, err := rp.CreateLink(tc.ctx(), tc.chatID, tc.url, tc.tags, tc.filters)
			if err != nil {
				require.ErrorIs(t, err, tc.err)
				assert.Nil(t, link)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, link)
			assert.Positive(t, link.ID)
			assert.Equal(t, tc.url, link.URL)
			assert.Equal(t, tc.tags, link.Tags)
			assert.Equal(t, tc.filters, link.Filters)
		})
	}
}

func TestLinkRepository_GetLinksByChatID(t *testing.T) {
	type testCase struct {
		name    string
		chatID  int64
		wantN   int
		ctx     func() context.Context
		prepare func(rp *LinkRepository)
		err     error
	}

	testCases := []testCase{
		{
			name:   "positive 1",
			chatID: 1,
			wantN:  2,
			ctx:    context.Background,
			prepare: func(rp *LinkRepository) {
				_, _ = rp.CreateLink(context.Background(), 1, "http://example.com", []string{"t1"}, []string{"f1"})
				_, _ = rp.CreateLink(context.Background(), 1, "http://example.org", []string{"t2"}, []string{"f2"})
			},
			err: nil,
		},
		{
			name:   "negative 1 chat not found",
			chatID: 999,
			wantN:  0,
			ctx:    context.Background,
			err:    usecase.ErrChatNotFound,
		},
		{
			name:   "negative 2 ctx is Done",
			chatID: 2,
			wantN:  0,
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			err: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp := NewLinkRepository().(*LinkRepository)

			if tc.prepare != nil {
				tc.prepare(rp)
			}

			links, err := rp.GetLinksByChatID(tc.ctx(), tc.chatID, 100, 0)
			if err != nil {
				require.ErrorIs(t, err, tc.err)
				assert.Nil(t, links)
				return
			}

			require.NoError(t, err)
			assert.Len(t, links, tc.wantN)
			for _, l := range links {
				assert.NotNil(t, l)
				assert.Positive(t, l.ID)
				assert.NotEmpty(t, l.URL)
			}
		})
	}
}

func TestLinkRepository_DeleteLink(t *testing.T) {
	type testCase struct {
		name    string
		chatID  int64
		url     string
		ctx     func() context.Context
		prepare func(rp *LinkRepository)
		err     error
	}

	testCases := []testCase{
		{
			name:   "positive 1",
			chatID: 1,
			url:    "http://example.com",
			ctx:    context.Background,
			prepare: func(rp *LinkRepository) {
				_, _ = rp.CreateLink(context.Background(), 1, "http://example.com", []string{"t1"}, []string{"f1"})
			},
			err: nil,
		},
		{
			name:   "negative 1 link not found",
			chatID: 1,
			url:    "http://nope.com",
			ctx:    context.Background,
			prepare: func(rp *LinkRepository) {
				_, _ = rp.CreateLink(context.Background(), 1, "http://example.com", []string{"t1"}, []string{"f1"})
			},
			err: usecase.ErrLinkNotFound,
		},
		{
			name:   "negative 2 chat not found",
			chatID: 999,
			url:    "http://example.com",
			ctx:    context.Background,
			err:    usecase.ErrChatNotFound,
		},
		{
			name:   "negative 3 ctx is Done",
			chatID: 2,
			url:    "http://example.net",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			err: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp := NewLinkRepository().(*LinkRepository)

			if tc.prepare != nil {
				tc.prepare(rp)
			}

			deleted, err := rp.DeleteLink(tc.ctx(), tc.chatID, tc.url)
			if err != nil {
				require.ErrorIs(t, err, tc.err)
				assert.Nil(t, deleted)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, deleted)
			assert.Equal(t, tc.url, deleted.URL)
			assert.Positive(t, deleted.ID)
		})
	}
}

func TestLinkRepository_ListLinks(t *testing.T) {
	type testCase struct {
		name    string
		ctx     func() context.Context
		prepare func(rp *LinkRepository)
		want    []string
		err     error
	}

	testCases := []testCase{
		{
			name: "positive empty",
			ctx:  context.Background,
			want: []string{},
			err:  nil,
		},
		{
			name: "positive sorted unique urls",
			ctx:  context.Background,
			prepare: func(rp *LinkRepository) {
				_, _ = rp.CreateLink(context.Background(), 2, "http://example.org", []string{"t2"}, []string{"f2"})
				_, _ = rp.CreateLink(context.Background(), 1, "http://example.com", []string{"t1"}, []string{"f1"})
				_, _ = rp.CreateLink(context.Background(), 3, "http://example.com", []string{"t3"}, []string{"f3"})
			},
			want: []string{"http://example.com", "http://example.org"},
			err:  nil,
		},
		{
			name: "negative ctx is Done",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			err: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp := NewLinkRepository().(*LinkRepository)

			if tc.prepare != nil {
				tc.prepare(rp)
			}

			links, err := rp.ListLinksBatch(tc.ctx(), 1000, 0)
			if err != nil {
				require.ErrorIs(t, err, tc.err)
				assert.Nil(t, links)
				return
			}

			require.NoError(t, err)
			require.Len(t, links, len(tc.want))

			for i, link := range links {
				require.NotNil(t, link)
				assert.Equal(t, tc.want[i], link.URL)
			}
		})
	}
}

func TestLinkRepository_GetChatIDsByLink(t *testing.T) {
	type testCase struct {
		name    string
		url     string
		ctx     func() context.Context
		prepare func(rp *LinkRepository)
		want    []int64
		err     error
	}

	testCases := []testCase{
		{
			name: "positive sorted ids",
			url:  "http://example.com",
			ctx:  context.Background,
			prepare: func(rp *LinkRepository) {
				_, _ = rp.CreateLink(context.Background(), 5, "http://example.com", []string{"t1"}, []string{"f1"})
				_, _ = rp.CreateLink(context.Background(), 2, "http://example.com", []string{"t2"}, []string{"f2"})
				_, _ = rp.CreateLink(context.Background(), 3, "http://example.com", []string{"t3"}, []string{"f3"})
			},
			want: []int64{2, 3, 5},
			err:  nil,
		},
		{
			name: "negative link not found",
			url:  "http://missing.com",
			ctx:  context.Background,
			err:  usecase.ErrLinkNotFound,
		},
		{
			name: "negative ctx is Done",
			url:  "http://example.com",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			err: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp := NewLinkRepository().(*LinkRepository)

			if tc.prepare != nil {
				tc.prepare(rp)
			}

			ids, err := rp.GetChatIDsByLink(tc.ctx(), tc.url)
			if err != nil {
				require.ErrorIs(t, err, tc.err)
				assert.Nil(t, ids)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, ids)
		})
	}
}

func TestLinkRepository_URLState(t *testing.T) {
	type testCase struct {
		name      string
		url       string
		initial   func(rp *LinkRepository)
		setCtx    func() context.Context
		getCtx    func() context.Context
		state     domain.URLState
		wantState domain.URLState
		setErr    error
		getErr    error
	}

	testCases := []testCase{
		{
			name: "positive set and get state",
			url:  "http://example.com",
			initial: func(rp *LinkRepository) {
				_, _ = rp.CreateLink(context.Background(), 1, "http://example.com", []string{"t1"}, []string{"f1"})
			},
			setCtx: context.Background,
			getCtx: context.Background,
			state: domain.URLState{
				LastCheckedAt: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
				LastUpdatedAt: time.Date(2026, 3, 30, 11, 0, 0, 0, time.UTC),
			},
			wantState: domain.URLState{
				LastCheckedAt: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
				LastUpdatedAt: time.Date(2026, 3, 30, 11, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "positive get zero state for unknown url",
			url:       "http://missing.com",
			setCtx:    context.Background,
			getCtx:    context.Background,
			wantState: domain.URLState{},
		},
		{
			name:   "negative set state link not found",
			url:    "http://missing.com",
			setCtx: context.Background,
			getCtx: context.Background,
			state: domain.URLState{
				LastCheckedAt: time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC),
				LastUpdatedAt: time.Date(2026, 3, 30, 13, 0, 0, 0, time.UTC),
			},
			setErr: usecase.ErrLinkNotFound,
		},
		{
			name: "negative set ctx is Done",
			url:  "http://example.com",
			initial: func(rp *LinkRepository) {
				_, _ = rp.CreateLink(context.Background(), 1, "http://example.com", []string{"t1"}, []string{"f1"})
			},
			setCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			getCtx: context.Background,
			state: domain.URLState{
				LastCheckedAt: time.Date(2026, 3, 30, 14, 0, 0, 0, time.UTC),
				LastUpdatedAt: time.Date(2026, 3, 30, 15, 0, 0, 0, time.UTC),
			},
			setErr: context.Canceled,
		},
		{
			name: "negative get ctx is Done",
			url:  "http://example.com",
			getCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			getErr: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp := NewLinkRepository().(*LinkRepository)

			if tc.initial != nil {
				tc.initial(rp)
			}

			if tc.state != (domain.URLState{}) || tc.setErr != nil {
				err := rp.SetURLState(tc.setCtx(), tc.url, tc.state)
				if tc.setErr != nil {
					require.ErrorIs(t, err, tc.setErr)
				} else {
					require.NoError(t, err)
				}
			}

			got, err := rp.GetURLState(tc.getCtx(), tc.url)
			if tc.getErr != nil {
				assert.ErrorIs(t, err, tc.getErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantState, got)
		})
	}
}
