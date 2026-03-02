package scrapper_app

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	grpcruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type Scrapper struct {
	server pbv1.ScrapperServer
	log    *zap.Logger
}

func New(server pbv1.ScrapperServer, log *zap.Logger) Scrapper {
	return Scrapper{
		server: server,
		log:    log,
	}
}

func (s *Scrapper) Run(ctx context.Context) {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGILL, syscall.SIGTERM)
	defer cancel()

	go s.runGrpc()
	go s.runRest(ctx)
	<-ctx.Done()
	time.Sleep(time.Second * 3)
}

func (s *Scrapper) runRest(ctx context.Context) {
	mux := grpcruntime.NewServeMux(
		grpcruntime.WithIncomingHeaderMatcher(func(k string) (string, bool) {
			if strings.EqualFold(k, "Tg-Chat-Id") {
				return "tg-chat-id", true
			}
			return grpcruntime.DefaultHeaderMatcher(k)
		}))
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	address := "localhost:" + "50051"
	err := pbv1.RegisterScrapperHandlerFromEndpoint(ctx, mux, address, opts)
	if err != nil {
		s.log.Error("can not register grpc gateway", zap.Error(err))
		os.Exit(-1)
	}

	gatewayPort := ":" + "8080"
	s.log.Info("gateway listening at port", zap.String("port", gatewayPort))

	if err = http.ListenAndServe(gatewayPort, mux); err != nil {
		s.log.Error("gateway listen error", zap.Error(err))
	}
}

func (s *Scrapper) runGrpc() {
	port := ":" + "50051"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		s.log.Error("can open tcp socker", zap.Error(err))
		os.Exit(-1)
	}
	srv := grpc.NewServer()
	reflection.Register(srv)
	pbv1.RegisterScrapperServer(srv, s.server)

	s.log.Info("grpc server listening at port", zap.String("port", port))
	if err = srv.Serve(lis); err != nil {
		s.log.Error("grpc server listen error", zap.Error(err))
	}
}
