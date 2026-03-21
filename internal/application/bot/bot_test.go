package botapp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := NewMockBotRepository(ctrl)
			bot, err := NewBot(tc.token, repo, nil, tc.router, zap.NewNop(), nil)

			if tc.positive {
				assert.NoError(t, err)
				assert.NotNil(t, bot)
				return
			}
			assert.Error(t, err)
			assert.Nil(t, bot)
			assert.EqualError(t, err, tc.err.Error())
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
		CommandHelp:  handlers.NewHelpHandler(),
		CommandStart: stubHandler{reply: "Привет! Я link-tracker бот. Напиши /help"},
	})
	handler := handlers.HelpHandler{}
	ans, _ := handler.Handle(1, "")
	testCases := []TestCase{
		{
			name: "positive 1 - help command is dispatched and replied",
			ctx: func() context.Context {
				return context.Background()
			},
			initMock: func(repo *MockBotRepository, updates chan domain.Message) {
				repo.EXPECT().GetMessages(gomock.Any(), 60).Return((<-chan domain.Message)(updates), nil)
				repo.EXPECT().SendMessage(int64(42), 7, ans).Return(nil)
			},
			feed: func(updates chan domain.Message) {
				updates <- domain.Message{ChatID: 42, MessageID: 7, Text: "/help"}
				close(updates)
			},
			positive: true,
		},
		{
			name: "positive 2 - whitespace message is ignored",
			ctx: func() context.Context {
				return context.Background()
			},
			initMock: func(repo *MockBotRepository, updates chan domain.Message) {
				repo.EXPECT().GetMessages(gomock.Any(), 60).Return((<-chan domain.Message)(updates), nil)
				repo.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			feed: func(updates chan domain.Message) {
				updates <- domain.Message{ChatID: 42, MessageID: 7, Text: "   \n\t"}
				close(updates)
			},
			positive: false,
			expected: ErrEmptyText,
		},
		{
			name: "positive 3 - unknown command returns fallback reply and is sent",
			ctx: func() context.Context {
				return context.Background()
			},
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
			ctx: func() context.Context {
				return context.Background()
			},
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
			ctx: func() context.Context {
				return context.Background()
			},
			initMock: func(repo *MockBotRepository, updates chan domain.Message) {
				repo.EXPECT().GetMessages(gomock.Any(), 60).Return((<-chan domain.Message)(updates), nil)
				repo.EXPECT().SendMessage(int64(42), 7, ans).Return(errors.New("send error"))
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
				repo.EXPECT().GetMessages(gomock.Any(), 60).Times(0)
				repo.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				_ = len(updates)
			},
			feed: func(updates chan domain.Message) {
				close(updates)
			},
			positive: false,
			expected: context.Canceled,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			updates := make(chan domain.Message, 4)
			repo := NewMockBotRepository(ctrl)
			tc.initMock(repo, updates)

			bot, err := NewBot("TEST_TOKEN", repo, nil, router, zap.NewNop(), &config.BotConfig{
				TokenTGBot:       "",
				ScrapperAddrGRPC: "",
				BotAddrGRPC:      "",
				BotAddrHTTP:      "",
			})
			assert.NoError(t, err)
			assert.NotNil(t, bot)

			go tc.feed(updates)

			runErr := bot.Run(tc.ctx())

			if tc.positive {
				if runErr != nil {
					assert.Error(t, runErr, "err in positive test")
				}
				assert.NoError(t, runErr)
				return
			}

			assert.Error(t, fmt.Errorf("expected err"))
			assert.Error(t, runErr)
			if tc.expected != nil {
				assert.EqualError(t, runErr, tc.expected.Error())
			}
		})
	}
}

type fakeBotGateway struct {
	mu   sync.Mutex
	sent []string
}

func (g *fakeBotGateway) GetMessages(_ context.Context, _ int) (<-chan domain.Message, error) {
	ch := make(chan domain.Message)
	close(ch)
	return ch, nil
}

func (g *fakeBotGateway) SendMessage(_ int64, _ int, text string) error {
	g.mu.Lock()
	g.sent = append(g.sent, text)
	g.mu.Unlock()
	return nil
}

func (g *fakeBotGateway) SentAll() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return strings.Join(g.sent, "\n")
}

type scrapperTestServer struct {
	pbv1.UnimplementedScrapperServer

	mu sync.Mutex

	createCalls []*pbv1.CreateLinkRequest
	createErr   error

	listResp *pbv1.ListLinksResponse
	listErr  error
}

func (s *scrapperTestServer) CreateLink(_ context.Context, req *pbv1.CreateLinkRequest) (*pbv1.LinkResponse, error) {
	s.mu.Lock()
	s.createCalls = append(s.createCalls, req)
	err := s.createErr
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	return &pbv1.LinkResponse{
		Url:     req.GetLink(),
		Tags:    req.GetTags(),
		Filters: req.GetFilters(),
	}, nil
}

func (s *scrapperTestServer) GetLinks(_ context.Context, _ *pbv1.GetLinksRequest) (*pbv1.ListLinksResponse, error) {
	s.mu.Lock()
	resp := s.listResp
	err := s.listErr
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if resp == nil {
		return &pbv1.ListLinksResponse{Links: nil, Size: 0}, nil
	}
	return resp, nil
}

func (s *scrapperTestServer) CreateCalls() []*pbv1.CreateLinkRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*pbv1.CreateLinkRequest, len(s.createCalls))
	copy(out, s.createCalls)
	return out
}

func newBufconnScrapperClient(t *testing.T, srv pbv1.ScrapperServer) (pbv1.ScrapperClient, func()) {
	t.Helper()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	gs := grpc.NewServer()
	pbv1.RegisterScrapperServer(gs, srv)

	go func() { _ = gs.Serve(lis) }()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		gs.Stop()
		_ = lis.Close()
	}
	return pbv1.NewScrapperClient(conn), cleanup
}

/*
Тесты, которые просились в тз. Не знаю как их выделить иначе кроме комента этого
*/
func TestBot_Track_And_List_TableDriven(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	type step struct{ text string }

	tests := []struct {
		name        string
		steps       []step
		scrapperCfg func(s *scrapperTestServer)
		wantSubstr  []string
		wantCreates int
		wantLink    string
		wantTags    []string
	}{
		{
			name: "track_valid_url_with_tags",
			steps: []step{
				{text: "/track"},
				{text: "https://github.com/user/repo"},
				{text: "tag1, tag2"},
			},
			scrapperCfg: func(_ *scrapperTestServer) {},
			wantSubstr: []string{
				"Пришли ссылку для отслеживания",
				"Теперь теги (необязательно)",
				"Начал отслеживать: https://github.com/user/repo",
				"Теги: tag1, tag2",
			},
			wantCreates: 1,
			wantLink:    "https://github.com/user/repo",
			wantTags:    []string{"tag1", "tag2"},
		},
		{
			name: "track_invalid_url",
			steps: []step{
				{text: "/track"},
				{text: "tbank://github.com/user/repo"},
			},
			wantSubstr: []string{
				"Пришли ссылку для отслеживания",
				"Некорректная ссылка. Пример: https://example.com",
			},
			wantCreates: 0,
		},
		{
			name: "track_already_exists",
			steps: []step{
				{text: "/track"},
				{text: "https://github.com/user/repo"},
				{text: "/skip"},
			},
			scrapperCfg: func(s *scrapperTestServer) {
				s.createErr = status.Error(codes.AlreadyExists, "already")
			},
			wantSubstr: []string{
				"Пришли ссылку для отслеживания",
				"Теперь теги (необязательно)",
				"Ссылка уже отслеживается",
			},
			wantCreates: 1,
			wantLink:    "https://github.com/user/repo",
		},
		{
			name:  "list_has_links",
			steps: []step{{text: "/list"}},
			scrapperCfg: func(s *scrapperTestServer) {
				s.listResp = &pbv1.ListLinksResponse{
					Links: []*pbv1.LinkResponse{
						{Url: "https://example.com/a", Tags: []string{"go"}},
						{Url: "https://example.com/b", Tags: []string{"java"}},
					},
					Size: 2,
				}
			},
			wantSubstr: []string{
				"Отслеживаемые ссылки:",
				"1) https://example.com/a",
				"2) https://example.com/b",
			},
			wantCreates: 0,
		},
		{
			name:  "list_empty_notfound",
			steps: []step{{text: "/list"}},
			scrapperCfg: func(s *scrapperTestServer) {
				s.listErr = status.Error(codes.NotFound, "no links")
			},
			wantSubstr: []string{
				"Список отслеживаемых ссылок пуст.",
			},
			wantCreates: 0,
		},
		{
			name:  "list_by_tag",
			steps: []step{{text: "/list go"}},
			scrapperCfg: func(s *scrapperTestServer) {
				s.listResp = &pbv1.ListLinksResponse{
					Links: []*pbv1.LinkResponse{
						{Url: "https://example.com/a", Tags: []string{"go"}},
						{Url: "https://example.com/b", Tags: []string{"java"}},
						{Url: "https://example.com/c", Tags: []string{"go", "backend"}},
					},
					Size: 3,
				}
			},
			wantSubstr: []string{
				`Ссылки с тегом "go":`,
				"1) https://example.com/a",
				"2) https://example.com/c",
			},
			wantCreates: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			botGw := &fakeBotGateway{}
			srv := &scrapperTestServer{}
			if tt.scrapperCfg != nil {
				tt.scrapperCfg(srv)
			}

			client, cleanup := newBufconnScrapperClient(t, srv)
			defer cleanup()

			router := NewBotDispatcher(map[domain.Command]domain.Handler{
				CommandTrack: handlers.NewTrackHandler(ctx, client),
				CommandList:  handlers.NewListHandler(ctx, client),
				CommandHelp:  handlers.NewHelpHandler(),
				CommandStart: stubHandler{reply: "Привет! Я link-tracker бот. Напиши /help"},
			})

			b := &Bot{
				botRepository: botGw,
				server:        nil,
				router:        router,
				log:           zap.NewNop(),
				cfg:           nil,
				fsm:           newFSMStore(),
			}

			for i, st := range tt.steps {
				_, err := b.handleIncomingMessage(domain.Message{
					ChatID:    1,
					MessageID: i + 1,
					Text:      st.text,
				}, true)
				if err != nil && !errors.Is(err, ErrorUnknownCommand) {
					t.Fatalf("step %d (%q) err: %v", i, st.text, err)
				}
			}

			all := botGw.SentAll()
			for _, sub := range tt.wantSubstr {
				if !strings.Contains(all, sub) {
					t.Fatalf("expected replies to contain %q, got:\n%s", sub, all)
				}
			}

			calls := srv.CreateCalls()
			if len(calls) != tt.wantCreates {
				t.Fatalf("expected CreateLink calls=%d, got=%d", tt.wantCreates, len(calls))
			}
			if tt.wantCreates > 0 && tt.wantLink != "" {
				if calls[0].GetLink() != tt.wantLink {
					t.Fatalf("expected CreateLink link=%q, got=%q", tt.wantLink, calls[0].GetLink())
				}
			}
			if tt.wantCreates > 0 && tt.wantTags != nil {
				got := calls[0].GetTags()
				if strings.Join(got, ",") != strings.Join(tt.wantTags, ",") {
					t.Fatalf("expected CreateLink tags=%v, got=%v", tt.wantTags, got)
				}
			}
		})
	}
}
