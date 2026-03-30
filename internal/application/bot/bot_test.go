package botapp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

func TestNewBot(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		name     string
		token    string
		router   *BotDispatcher
		positive bool
		err      error
	}

	testCases := []TestCase{
		{
			name:     "positive 1",
			token:    "TEST_TOKEN",
			router:   NewBotDispatcher(nil),
			positive: true,
		},
		{
			name:     "negative 1",
			token:    "",
			router:   NewBotDispatcher(nil),
			positive: false,
			err:      errors.New("empty telegram token"),
		},
		{
			name:     "negative 2",
			token:    "TEST_TOKEN",
			router:   nil,
			positive: false,
			err:      errors.New("nil router"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := NewMockBotRepository(ctrl)
			bot, err := NewBot(tc.token, repo, tc.router, zap.NewNop())

			if tc.positive {
				require.NoError(t, err)
				assert.NotNil(t, bot)
				return
			}
			require.Error(t, err)
			assert.Nil(t, bot)
			require.EqualError(t, err, tc.err.Error())
		})
	}
}

func TestBot_Run(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		name     string
		ctx      func() context.Context
		initMock func(repo *MockBotRepository, updates chan domain.Message)
		feed     func(updates chan domain.Message)
		positive bool
		expected error
	}

	router := NewBotDispatcher(map[domain.Command]domain.Handler{
		CommandHelp:  NewHelpHandler(),
		CommandStart: NewStartHandler(),
	})

	testCases := []TestCase{
		{
			name: "positive 1 - help command is dispatched and replied",
			ctx:  context.Background,
			initMock: func(repo *MockBotRepository, updates chan domain.Message) {
				repo.EXPECT().GetMessages(gomock.Any(), 60).Return((<-chan domain.Message)(updates), nil)
				repo.EXPECT().SendMessage(int64(42), 7, "/start — начать\n/help — список команд").Return(nil)
			},
			feed: func(updates chan domain.Message) {
				updates <- domain.Message{ChatID: 42, MessageID: 7, Text: "/help"}
				close(updates)
			},
			positive: true,
		},
		{
			name: "positive 2 - whitespace message is ignored",
			ctx:  context.Background,
			initMock: func(repo *MockBotRepository, updates chan domain.Message) {
				repo.EXPECT().GetMessages(gomock.Any(), 60).Return((<-chan domain.Message)(updates), nil)
				repo.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			feed: func(updates chan domain.Message) {
				updates <- domain.Message{ChatID: 42, MessageID: 7, Text: "   \n\t"}
				close(updates)
			},
			positive: true,
		},
		{
			name: "positive 3 - unknown command returns fallback reply and is sent",
			ctx:  context.Background,
			initMock: func(repo *MockBotRepository, updates chan domain.Message) {
				repo.EXPECT().GetMessages(gomock.Any(), 60).Return((<-chan domain.Message)(updates), nil)
				repo.EXPECT().SendMessage(int64(1), 2, "Не знаю команду /unknown. Напиши /help").Return(nil)
			},
			feed: func(updates chan domain.Message) {
				updates <- domain.Message{ChatID: 1, MessageID: 2, Text: "/unknown"}
				close(updates)
			},
			positive: true,
		},
		{
			name: "negative 1 - GetMessages returns error",
			ctx:  context.Background,
			initMock: func(repo *MockBotRepository, updates chan domain.Message) {
				_ = updates
				repo.EXPECT().GetMessages(gomock.Any(), 60).Return((<-chan domain.Message)(nil), errors.New("get messages error"))
			},
			feed: func(updates chan domain.Message) {
				close(updates)
			},
			positive: false,
			expected: errors.New("get messages error"),
		},
		{
			name: "negative 2 - SendMessage returns error",
			ctx:  context.Background,
			initMock: func(repo *MockBotRepository, updates chan domain.Message) {
				repo.EXPECT().GetMessages(gomock.Any(), 60).Return((<-chan domain.Message)(updates), nil)
				repo.EXPECT().SendMessage(int64(42), 7, "/start — начать\n/help — список команд").Return(errors.New("send error"))
			},
			feed: func(updates chan domain.Message) {
				updates <- domain.Message{ChatID: 42, MessageID: 7, Text: "/help"}
				close(updates)
			},
			positive: false,
			expected: errors.New("send error"),
		},
		{
			name: "negative 3 - ctx cancelled",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			initMock: func(repo *MockBotRepository, updates chan domain.Message) {
				repo.EXPECT().GetMessages(gomock.Any(), 60).Return((<-chan domain.Message)(updates), nil)
				repo.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			feed: func(updates chan domain.Message) {
				close(updates)
			},
			positive: false,
			expected: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			updates := make(chan domain.Message, 4)
			repo := NewMockBotRepository(ctrl)
			tc.initMock(repo, updates)

			bot, err := NewBot("TEST_TOKEN", repo, router, zap.NewNop())
			require.NoError(t, err)
			assert.NotNil(t, bot)

			go tc.feed(updates)

			runErr := bot.Run(tc.ctx())

			if tc.positive {
				require.NoError(t, runErr)
				return
			}

			require.Error(t, runErr)
			if tc.expected != nil {
				require.ErrorContains(t, runErr, tc.expected.Error())
			}
		})
	}
}
