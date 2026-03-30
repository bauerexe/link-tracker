package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
)

func TestChatRepository_CreateChat(t *testing.T) {
	type testCase struct {
		name    string
		id      int64
		ctx     func() context.Context
		prepare func(rp usecase.ChatRepository)
		err     error
	}

	testCases := []testCase{
		{
			name: "positive 1",
			id:   123,
			ctx:  context.Background,
			err:  nil,
		},
		{
			name: "negative 1",
			id:   123,
			ctx:  context.Background,
			prepare: func(rp usecase.ChatRepository) {
				_, _ = rp.CreateChat(context.Background(), 123)
			},
			err: usecase.ErrChatAlreadyExist,
		},
		{
			name: "negative 2 ctx is Done",
			id:   124,
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
			rp := NewChatRepository()

			if tc.prepare != nil {
				tc.prepare(rp)
			}

			chat, err := rp.CreateChat(tc.ctx(), tc.id)

			switch {
			case tc.err != nil:
				require.ErrorIs(t, err, tc.err)
			default:
				require.NoError(t, err)
				require.Equal(t, tc.id, chat.ID)
			}
		})
	}
}

func TestChatRepository_DeleteChatByID(t *testing.T) {
	type testCase struct {
		name    string
		id      int64
		ctx     func() context.Context
		prepare func(rp usecase.ChatRepository)
		err     error
	}

	testCases := []testCase{
		{
			name: "positive 1",
			id:   123,
			ctx:  context.Background,
			prepare: func(rp usecase.ChatRepository) {
				_, _ = rp.CreateChat(context.Background(), 123)
			},
			err: nil,
		},
		{
			name: "negative 1 chat not found",
			id:   123,
			ctx:  context.Background,
			err:  usecase.ErrChatNotFound,
		},
		{
			name: "negative 2 ctx is Done",
			id:   124,
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			prepare: func(rp usecase.ChatRepository) {
				_, _ = rp.CreateChat(context.Background(), 123)
			},
			err: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp := NewChatRepository()

			if tc.prepare != nil {
				tc.prepare(rp)
			}

			chat, err := rp.DeleteChatByID(tc.ctx(), tc.id)

			switch {
			case tc.err != nil:
				require.ErrorIs(t, err, tc.err)
			default:
				require.NoError(t, err)
				require.Equal(t, tc.id, chat.ID)
			}
		})
	}
}

func TestChatRepository_GetChatByID(t *testing.T) {
	type testCase struct {
		name    string
		id      int64
		ctx     func() context.Context
		prepare func(rp usecase.ChatRepository)
		err     error
	}

	testCases := []testCase{
		{
			name: "positive 1",
			id:   123,
			ctx:  context.Background,
			prepare: func(rp usecase.ChatRepository) {
				_, _ = rp.CreateChat(context.Background(), 123)
			},
			err: nil,
		},
		{
			name: "negative 1 chat not found",
			id:   999,
			ctx:  context.Background,
			err:  usecase.ErrChatNotFound,
		},
		{
			name: "negative 2 ctx is Done",
			id:   124,
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			prepare: func(rp usecase.ChatRepository) {
				_, _ = rp.CreateChat(context.Background(), 123)
			},
			err: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp := NewChatRepository()

			if tc.prepare != nil {
				tc.prepare(rp)
			}

			chat, err := rp.GetChatByID(tc.ctx(), tc.id)

			switch {
			case tc.err != nil:
				require.ErrorIs(t, err, tc.err)
			default:
				require.NoError(t, err)
				require.Equal(t, tc.id, chat.ID)
			}
		})
	}
}
