package scrapper_app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	grpcruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/config"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type dummyScrapperServer struct {
	pbv1.UnimplementedScrapperServer
}

type exitPanic struct {
	code int
}

/*
Тесты, которые просились в тз. Не знаю как их выделить иначе кроме комента этого
*/
func TestScrapper_runRest_registerError_exits(t *testing.T) {
	oldRegister := RegisterScrapperGateway
	oldExit := ExitFn
	oldListen := HttpListenAndServe

	defer func() {
		RegisterScrapperGateway = oldRegister
		ExitFn = oldExit
		HttpListenAndServe = oldListen
	}()

	exitCalled := false
	exitCode := 0
	ExitFn = func(code int) {
		exitCalled = true
		exitCode = code
		panic(exitPanic{code: code})
	}

	RegisterScrapperGateway = func(_ context.Context, _ *grpcruntime.ServeMux, _ string, _ []grpc.DialOption) error {
		return errors.New("boom")
	}

	HttpListenAndServe = func(_ string, _ http.Handler) error {
		return nil
	}

	s := New(&dummyScrapperServer{}, zap.NewNop(), &config.ScrapperConfig{
		ScrapperAddrGRPC: "localhost:50051",
		ScrapperAddrHTTP: "localhost:0",
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected exit panic")
		}
		if _, ok := r.(exitPanic); !ok {
			t.Fatalf("unexpected panic: %v", r)
		}
		if !exitCalled || exitCode != -1 {
			t.Fatalf("expected exit(-1), got called=%v code=%d", exitCalled, exitCode)
		}
	}()

	s.runRest(context.Background())
}

func TestScrapper_runRest_listenCalled(t *testing.T) {
	oldRegister := RegisterScrapperGateway
	oldExit := ExitFn
	oldListen := HttpListenAndServe

	defer func() {
		RegisterScrapperGateway = oldRegister
		ExitFn = oldExit
		HttpListenAndServe = oldListen
	}()

	ExitFn = func(_ int) {}

	RegisterScrapperGateway = func(_ context.Context, _ *grpcruntime.ServeMux, _ string, _ []grpc.DialOption) error {
		return nil
	}

	called := false
	gotAddr := ""
	HttpListenAndServe = func(addr string, _ http.Handler) error {
		called = true
		gotAddr = addr
		return errors.New("listen error")
	}

	s := New(&dummyScrapperServer{}, zap.NewNop(), &config.ScrapperConfig{
		ScrapperAddrGRPC: "localhost:50051",
		ScrapperAddrHTTP: "localhost:8080",
	})

	s.runRest(context.Background())

	if !called || gotAddr != "localhost:8080" {
		t.Fatalf("expected listen called with localhost:8080, got called=%v addr=%q", called, gotAddr)
	}
}

func TestScrapper_runGrpc_listenError_exits(t *testing.T) {
	oldListen := NetListen
	oldExit := ExitFn

	defer func() {
		NetListen = oldListen
		ExitFn = oldExit
	}()

	exitCalled := false
	exitCode := 0
	ExitFn = func(code int) {
		exitCalled = true
		exitCode = code
		panic(exitPanic{code: code})
	}

	NetListen = func(_, _ string) (net.Listener, error) {
		return nil, errors.New("listen fail")
	}

	s := New(&dummyScrapperServer{}, zap.NewNop(), &config.ScrapperConfig{
		ScrapperAddrGRPC: "localhost:50051",
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected exit panic")
		}
		if _, ok := r.(exitPanic); !ok {
			t.Fatalf("unexpected panic: %v", r)
		}
		if !exitCalled || exitCode != -1 {
			t.Fatalf("expected exit(-1), got called=%v code=%d", exitCalled, exitCode)
		}
	}()

	s.runGrpc()
}

func TestScrapper_runGrpc_netListenCalled(t *testing.T) {
	oldListen := NetListen
	oldExit := ExitFn

	defer func() {
		NetListen = oldListen
		ExitFn = oldExit
	}()

	listenCalled := false
	gotNetwork := ""
	gotAddress := ""

	ExitFn = func(code int) {
		panic(exitPanic{code: code})
	}

	NetListen = func(network, address string) (net.Listener, error) {
		listenCalled = true
		gotNetwork = network
		gotAddress = address
		return nil, errors.New("listen fail")
	}

	s := New(&dummyScrapperServer{}, zap.NewNop(), &config.ScrapperConfig{
		ScrapperAddrGRPC: "127.0.0.1:50051",
	})

	defer func() { _ = recover() }()

	s.runGrpc()

	if !listenCalled {
		t.Fatalf("expected NetListen to be called")
	}
	if gotNetwork != "tcp" || gotAddress != "127.0.0.1:50051" {
		t.Fatalf("expected NetListen called with tcp and 127.0.0.1:50051, got network=%q addr=%q", gotNetwork, gotAddress)
	}
}

type fakeLinksRepo struct {
	mu        sync.Mutex
	links     []*domain.Link
	chatByURL map[string][]int64
	state     map[string]domain.URLState
}

func (r *fakeLinksRepo) CreateLink(_ context.Context, _ int64, _ string, _, _ []string) (*domain.Link, error) {
	return &domain.Link{}, nil
}

func (r *fakeLinksRepo) GetLinksByChatID(_ context.Context, _ int64) ([]*domain.Link, error) {
	return nil, nil
}

func (r *fakeLinksRepo) DeleteLink(_ context.Context, _ int64, _ string) (*domain.Link, error) {
	return &domain.Link{}, nil
}

func (r *fakeLinksRepo) ListLinks(_ context.Context) ([]*domain.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.Link, len(r.links))
	copy(out, r.links)
	return out, nil
}

func (r *fakeLinksRepo) GetChatIDsByLink(_ context.Context, url string) ([]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := r.chatByURL[url]
	out := make([]int64, len(ids))
	copy(out, ids)
	return out, nil
}

func (r *fakeLinksRepo) GetURLState(_ context.Context, url string) (domain.URLState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		r.state = map[string]domain.URLState{}
	}
	return r.state[url], nil
}

func (r *fakeLinksRepo) SetURLState(_ context.Context, url string, st domain.URLState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		r.state = map[string]domain.URLState{}
	}
	r.state[url] = st
	return nil
}

type fakeNotifier struct {
	mu    sync.Mutex
	calls []struct {
		url  string
		desc string
		ids  []int64
	}
}

func (n *fakeNotifier) Notify(_ context.Context, url, description string, chatIDs []int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	ids := make([]int64, len(chatIDs))
	copy(ids, chatIDs)
	n.calls = append(n.calls, struct {
		url  string
		desc string
		ids  []int64
	}{url: url, desc: description, ids: ids})
	return nil
}

func (n *fakeNotifier) Calls() []struct {
	url  string
	desc string
	ids  []int64
} {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]struct {
		url  string
		desc string
		ids  []int64
	}, len(n.calls))
	copy(out, n.calls)
	return out
}

type fakeChecker struct {
	matchURL  string
	updated   bool
	desc      string
	updatedAt time.Time
	err       error
}

func (c *fakeChecker) Match(url string) bool { return url == c.matchURL }
func (c *fakeChecker) Check(context.Context, string, time.Time) (string, time.Time, bool, error) {
	return c.desc, c.updatedAt, c.updated, c.err
}

func TestScheduler_Recipients_OnlySubscribersGetUpdate(t *testing.T) {
	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)

	repo := &fakeLinksRepo{
		links: []*domain.Link{
			{URL: "https://example.com/url1"},
			{URL: "https://example.com/url2"},
		},
		chatByURL: map[string][]int64{
			"https://example.com/url1": {1, 3},
			"https://example.com/url2": {2},
		},
		state: map[string]domain.URLState{},
	}

	notifier := &fakeNotifier{}
	checker := &fakeChecker{
		matchURL:  "https://example.com/url1",
		updated:   true,
		desc:      "updated",
		updatedAt: now.Add(-time.Minute),
	}

	s, err := NewScheduler(&Scheduler{
		Links:    repo,
		Notifier: notifier,
		Checkers: []Checker{checker},
		Log:      zap.NewNop(),
		Interval: time.Minute,
	})
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}

	s.tickWithNow(context.Background(), now)

	calls := notifier.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected notify calls=1, got=%d", len(calls))
	}
	if calls[0].url != "https://example.com/url1" {
		t.Fatalf("expected url1, got=%q", calls[0].url)
	}
	if stringsJoinInt64(calls[0].ids) != stringsJoinInt64([]int64{1, 3}) {
		t.Fatalf("expected chatIDs [1 3], got=%v", calls[0].ids)
	}
}

func stringsJoinInt64(xs []int64) string {
	if len(xs) == 0 {
		return ""
	}
	var b strings.Builder
	for i, x := range xs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(int64ToString(x))
	}
	return b.String()
}

func int64ToString(x int64) string {
	if x == 0 {
		return "0"
	}
	neg := x < 0
	if neg {
		x = -x
	}
	var buf [32]byte
	i := len(buf)
	for x > 0 {
		i--
		buf[i] = byte('0' + x%10)
		x /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestScheduler_CheckerErrors_TableDriven(t *testing.T) {
	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		checkErr   error
		wantNotify bool
	}{
		{name: "non_2xx_error", checkErr: errors.New("bad status 500"), wantNotify: false},
		{name: "invalid_json_error", checkErr: errors.New("invalid json"), wantNotify: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeLinksRepo{
				links: []*domain.Link{
					{URL: "https://example.com/url1"},
				},
				chatByURL: map[string][]int64{
					"https://example.com/url1": {1},
				},
				state: map[string]domain.URLState{},
			}

			notifier := &fakeNotifier{}
			checker := &fakeChecker{
				matchURL:  "https://example.com/url1",
				updated:   false,
				desc:      "",
				updatedAt: now,
				err:       tt.checkErr,
			}

			s, err := NewScheduler(&Scheduler{
				Links:    repo,
				Notifier: notifier,
				Checkers: []Checker{checker},
				Log:      zap.NewNop(),
				Interval: time.Minute,
			})
			if err != nil {
				t.Fatalf("NewScheduler: %v", err)
			}

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("unexpected panic: %v", r)
				}
			}()

			s.tickWithNow(context.Background(), now)

			calls := notifier.Calls()
			if tt.wantNotify && len(calls) == 0 {
				t.Fatalf("expected notify, got none")
			}
			if !tt.wantNotify && len(calls) != 0 {
				t.Fatalf("expected no notify, got %d", len(calls))
			}
		})
	}
}
