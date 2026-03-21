package botapp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	grpcruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type dummyBotServer struct {
	pbv1.UnimplementedBotServer
}

type fakeListener struct{}

func (fakeListener) Accept() (net.Conn, error) { return nil, errors.New("closed") }
func (fakeListener) Close() error              { return nil }
func (fakeListener) Addr() net.Addr            { return &net.TCPAddr{} }

func TestBot_runRest_registerError_returns(t *testing.T) {
	oldRegister := RegisterBotGateway
	defer func() {
		RegisterBotGateway = oldRegister
	}()

	RegisterBotGateway = func(_ context.Context, _ *grpcruntime.ServeMux, _ string, _ []grpc.DialOption) error {
		return errors.New("boom")
	}

	b := &Bot{
		server: &dummyBotServer{},
		log:    zap.NewNop(),
		cfg: &config.BotConfig{
			BotAddrGRPC: "localhost:50052",
			BotAddrHTTP: "localhost:8082",
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	b.runRest(context.Background())
}

func TestBot_runRest_listenCalled(t *testing.T) {
	oldRegister := RegisterBotGateway
	oldExit := ExitFn
	oldNetListen := NetListen
	oldHTTPServe := HttpServe

	defer func() {
		RegisterBotGateway = oldRegister
		ExitFn = oldExit
		NetListen = oldNetListen
		HttpServe = oldHTTPServe
	}()

	ExitFn = func(_ int) {}

	RegisterBotGateway = func(_ context.Context, _ *grpcruntime.ServeMux, _ string, _ []grpc.DialOption) error {
		return nil
	}

	listenCalled := false
	serveCalled := false
	gotNetwork := ""
	gotAddr := ""

	ln := fakeListener{}

	NetListen = func(network, address string) (net.Listener, error) {
		listenCalled = true
		gotNetwork = network
		gotAddr = address
		return ln, nil
	}

	HttpServe = func(gotLn net.Listener, _ http.Handler) error {
		serveCalled = true
		if gotLn != ln {
			t.Fatalf("unexpected listener passed to HttpServe")
		}
		return errors.New("serve error")
	}

	b := &Bot{
		server: &dummyBotServer{},
		log:    zap.NewNop(),
		cfg: &config.BotConfig{
			BotAddrGRPC: "localhost:50052",
			BotAddrHTTP: "localhost:8082",
		},
	}

	b.runRest(context.Background())

	if !listenCalled {
		t.Fatalf("expected NetListen to be called")
	}
	if gotNetwork != "tcp" || gotAddr != "localhost:8082" {
		t.Fatalf("expected NetListen called with tcp localhost:8082, got network=%q addr=%q", gotNetwork, gotAddr)
	}
	if !serveCalled {
		t.Fatalf("expected HttpServe to be called")
	}
}
