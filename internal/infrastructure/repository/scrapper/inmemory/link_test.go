package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
)

func TestLinkRepository_CreateLink(t *testing.T) {
	type testCase struct {
		name    string
		chatID  int64
		url     string
		tags    []string
		filters []string
		err     error
	}

	testCases := []testCase{
		{
			name:    "positive 1",
			chatID:  1,
			url:     "http://example.com",
			tags:    []string{"t1"},
			filters: []string{"f1"},
			err:     nil,
		},
		{
			name:    "negative 1 already tracked",
			chatID:  1,
			url:     "http://example.com",
			tags:    []string{"t1"},
			filters: []string{"f1"},
			err:     usecase.ErrLinkAlreadyTracked,
		},
		{
			name:    "positive 2 another link same chat",
			chatID:  1,
			url:     "http://example.org",
			tags:    []string{"t2"},
			filters: []string{"f2"},
			err:     nil,
		},
		{
			name:    "negative 2 ctx is Done",
			chatID:  2,
			url:     "http://example.net",
			tags:    []string{"t3"},
			filters: []string{"f3"},
			err:     context.DeadlineExceeded,
		},
	}

	rp := NewLinkRepository()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*110)
	defer cancel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			link, err := rp.CreateLink(ctx, tc.chatID, tc.url, tc.tags, tc.filters)
			if err != nil {
				assert.ErrorIs(t, err, tc.err)
				return
			}

			if tc.err == nil {
				assert.NotNil(t, link)
				assert.Greater(t, link.ID, int64(0))
				assert.Equal(t, tc.url, link.URL)
				assert.Equal(t, tc.tags, link.Tags)
				assert.Equal(t, tc.filters, link.Filters)
				return
			}

			assert.Error(t, err)
		})

		time.Sleep(time.Millisecond * 50)
	}
}

func TestLinkRepository_GetLinksByChatID(t *testing.T) {
	type testCase struct {
		name   string
		chatID int64
		wantN  int
		err    error
	}

	testCases := []testCase{
		{
			name:   "positive 1",
			chatID: 1,
			wantN:  2,
			err:    nil,
		},
		{
			name:   "negative 1 chat not found",
			chatID: 999,
			wantN:  0,
			err:    usecase.ErrChatNotFound,
		},
		{
			name:   "negative 2 ctx is Done",
			chatID: 2,
			wantN:  0,
			err:    context.DeadlineExceeded,
		},
	}

	rp := NewLinkRepository()

	{
		ctxPrep := context.Background()
		_, err := rp.CreateLink(ctxPrep, 1, "http://example.com", []string{"t1"}, []string{"f1"})
		assert.NoError(t, err)
		_, err = rp.CreateLink(ctxPrep, 1, "http://example.org", []string{"t2"}, []string{"f2"})
		assert.NoError(t, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			links, err := rp.GetLinksByChatID(ctx, tc.chatID)
			if err != nil {
				assert.ErrorIs(t, err, tc.err)
				return
			}

			if tc.err == nil {
				assert.Len(t, links, tc.wantN)
				for _, l := range links {
					assert.Greater(t, l.ID, int64(0))
					assert.NotEmpty(t, l.URL)
				}
				return
			}

			assert.Error(t, err)
		})

		time.Sleep(time.Millisecond * 50)
	}
}

func TestLinkRepository_DeleteLink(t *testing.T) {
	type testCase struct {
		name   string
		chatID int64
		url    string
		err    error
	}

	testCases := []testCase{
		{
			name:   "positive 1",
			chatID: 1,
			url:    "http://example.com",
			err:    nil,
		},
		{
			name:   "negative 1 link not found",
			chatID: 1,
			url:    "http://nope.com",
			err:    usecase.ErrChatNotFound,
		},
		{
			name:   "negative 2 chat not found",
			chatID: 999,
			url:    "http://example.com",
			err:    usecase.ErrChatNotFound,
		},
		{
			name:   "negative 3 ctx is Done",
			chatID: 2,
			url:    "http://example.net",
			err:    context.DeadlineExceeded,
		},
	}

	rp := NewLinkRepository()

	ctxPrep := context.Background()
	_, err := rp.CreateLink(ctxPrep, 1, "http://example.com", []string{"t1"}, []string{"f1"})
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*110)
	defer cancel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deleted, err := rp.DeleteLink(ctx, tc.chatID, tc.url)
			if err != nil {
				assert.ErrorIs(t, err, tc.err)
				return
			}

			if tc.err == nil {
				assert.NotNil(t, deleted)
				assert.Equal(t, tc.url, deleted.URL)
				assert.Greater(t, deleted.ID, int64(0))
				return
			}

			assert.Error(t, err)
		})

		time.Sleep(time.Millisecond * 50)
	}
}
