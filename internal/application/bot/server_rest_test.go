package botapp

import (
	"context"
	"errors"
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

func TestBot_runRest_registerError_exits(t *testing.T) {
	oldRegister := RegisterBotGateway
	oldExit := BotExitFn
	oldListen := BotHTTPListenAndServe

	defer func() {
		RegisterBotGateway = oldRegister
		BotExitFn = oldExit
		BotHTTPListenAndServe = oldListen
	}()

	exitCalled := false
	exitCode := 0
	BotExitFn = func(code int) {
		exitCalled = true
		exitCode = code
	}

	RegisterBotGateway = func(_ context.Context, _ *grpcruntime.ServeMux, _ string, _ []grpc.DialOption) error {
		return errors.New("boom")
	}

	BotHTTPListenAndServe = func(_ string, _ http.Handler) error {
		return nil
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

	if !exitCalled || exitCode != -1 {
		t.Fatalf("expected exit(-1), got called=%v code=%d", exitCalled, exitCode)
	}
}

func TestBot_runRest_listenCalled(t *testing.T) {
	oldRegister := RegisterBotGateway
	oldExit := BotExitFn
	oldListen := BotHTTPListenAndServe

	defer func() {
		RegisterBotGateway = oldRegister
		BotExitFn = oldExit
		BotHTTPListenAndServe = oldListen
	}()

	BotExitFn = func(_ int) {}

	RegisterBotGateway = func(_ context.Context, _ *grpcruntime.ServeMux, _ string, _ []grpc.DialOption) error {
		return errors.New("boom")
	}

	called := false
	gotAddr := ""
	BotHTTPListenAndServe = func(addr string, _ http.Handler) error {
		called = true
		gotAddr = addr
		return errors.New("listen error")
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

	if !called || gotAddr != "localhost:8082" {
		t.Fatalf("expected listen called with localhost:8082, got called=%v addr=%q", called, gotAddr)
	}
}
