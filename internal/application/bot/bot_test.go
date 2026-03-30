package botapp

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repo := NewMockBotRepository(ctrl)
			bot, err := NewBot(tc.token, repo, nil, tc.router, zap.NewNop(), nil)

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

type step struct {
	text    string
	command string
	args    string
}

func TestBot_Track_And_List_TableDriven(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

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
				{text: "/track", command: "track"},
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
				{text: "/track", command: "track"},
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
				{text: "/track", command: "track"},
				{text: "https://github.com/user/repo"},
				{text: "/skip", command: "skip"},
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
			steps: []step{{text: "/list", command: "list"}},
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
			steps: []step{{text: "/list", command: "list"}},
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
			steps: []step{{text: "/list go", command: "list", args: "go"}},
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
		t.Run(tt.name, func(t *testing.T) {
			botGw, srv, b := newTrackListTestBot(ctx, t, tt.scrapperCfg)
			runBotSteps(t, b, tt.steps)
			assertBotRepliesContain(t, botGw.SentAll(), tt.wantSubstr)
			assertCreateCalls(t, srv.CreateCalls(), tt.wantCreates, tt.wantLink, tt.wantTags)
		})
	}
}

func newTrackListTestBot(ctx context.Context, t *testing.T, scrapperCfg func(s *scrapperTestServer)) (*fakeBotGateway, *scrapperTestServer, *Bot) {
	t.Helper()

	botGw := &fakeBotGateway{}
	srv := &scrapperTestServer{}
	if scrapperCfg != nil {
		scrapperCfg(srv)
	}

	client, cleanup := newBufconnScrapperClient(t, srv)
	t.Cleanup(cleanup)

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

	return botGw, srv, b
}

func runBotSteps(t *testing.T, b *Bot, steps []step) {
	t.Helper()

	for i, st := range steps {
		_, err := b.handleIncomingMessage(domain.Message{
			ChatID:    1,
			MessageID: i + 1,
			Text:      st.text,
			Command:   domain.Command(st.command),
			Arguments: st.args,
		}, true)
		if err != nil && !errors.Is(err, ErrUnknownCommand) {
			t.Fatalf("step %d (%q) err: %v", i, st.text, err)
		}
	}
}

func assertBotRepliesContain(t *testing.T, all string, wantSubstr []string) {
	t.Helper()

	for _, sub := range wantSubstr {
		if !strings.Contains(all, sub) {
			t.Fatalf("expected replies to contain %q, got:\n%s", sub, all)
		}
	}
}

func assertCreateCalls(t *testing.T, calls []*pbv1.CreateLinkRequest, wantCreates int, wantLink string, wantTags []string) {
	t.Helper()

	if len(calls) != wantCreates {
		t.Fatalf("expected CreateLink calls=%d, got=%d", wantCreates, len(calls))
	}

	if wantCreates == 0 {
		return
	}

	if wantLink != "" && calls[0].GetLink() != wantLink {
		t.Fatalf("expected CreateLink link=%q, got=%q", wantLink, calls[0].GetLink())
	}

	if wantTags != nil {
		got := calls[0].GetTags()
		if strings.Join(got, ",") != strings.Join(wantTags, ",") {
			t.Fatalf("expected CreateLink tags=%v, got=%v", wantTags, got)
		}
	}
}
