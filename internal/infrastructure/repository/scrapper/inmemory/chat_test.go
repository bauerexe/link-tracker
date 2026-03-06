package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
)

func TestChatRepository_CreateChat(t *testing.T) {
	type testCase struct {
		name string
		id   int64
		err  error
	}

	testCases := []testCase{
		{
			name: "positive 1",
			id:   123,
			err:  nil,
		},
		{
			name: "negative 1",
			id:   123,
			err:  usecase.ErrChatAlreadyExist,
		},
		{
			name: "negative 2 ctx is Done",
			id:   124,
			err:  context.DeadlineExceeded,
		},
	}
	rp := NewChatRepository()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chat, err := rp.CreateChat(ctx, tc.id)
			if err != nil {
				assert.ErrorIs(t, err, tc.err)
			} else if tc.err == nil {
				assert.Equal(t, chat.ID, tc.id)
			} else {
				assert.Error(t, err)
			}
			time.Sleep(time.Millisecond * 51)
		})
	}
}

func TestChatRepository_DeleteChatByID(t *testing.T) {
	type testCase struct {
		name string
		id   int64
		err  error
	}

	testCases := []testCase{
		{
			name: "positive 1",
			id:   123,
			err:  nil,
		},
		{
			name: "negative 1 chat not found",
			id:   123,
			err:  usecase.ErrChatNotFound,
		},
		{
			name: "negative 2 ctx is Done",
			id:   124,
			err:  context.DeadlineExceeded,
		},
	}

	rp := NewChatRepository()

	ctxPrep, cancel1 := context.WithTimeout(context.Background(), time.Second)
	defer cancel1()
	_, err := rp.CreateChat(ctxPrep, 123)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chat, err := rp.DeleteChatByID(ctx, tc.id)
			if err != nil {
				assert.ErrorIs(t, err, tc.err)
				return
			}

			if tc.err == nil {
				assert.Equal(t, tc.id, chat.ID)
				return
			}

			assert.Error(t, err)
		})

		time.Sleep(time.Millisecond * 51)
	}
}

func TestChatRepository_GetChatByID(t *testing.T) {
	type testCase struct {
		name string
		id   int64
		err  error
	}

	testCases := []testCase{
		{
			name: "positive 1",
			id:   123,
			err:  nil,
		},
		{
			name: "negative 1 chat not found",
			id:   999,
			err:  usecase.ErrChatNotFound,
		},
		{
			name: "negative 2 ctx is Done",
			id:   124,
			err:  context.DeadlineExceeded,
		},
	}

	rp := NewChatRepository()

	ctxPrep, cancel1 := context.WithTimeout(context.Background(), time.Second)
	defer cancel1()
	_, err := rp.CreateChat(ctxPrep, 123)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chat, err := rp.GetChatByID(ctx, tc.id)
			if err != nil {
				assert.ErrorIs(t, err, tc.err)
				return
			}

			if tc.err == nil {
				assert.Equal(t, tc.id, chat.ID)
				return
			}

			assert.Error(t, err)
		})

		time.Sleep(time.Millisecond * 51)
	}
}
