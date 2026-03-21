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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

// Scrapper - use case for service to keep info for scraping links
type Scrapper struct {
	server pbv1.ScrapperServer
	log    *zap.Logger
	cfg    *config.ScrapperConfig
}

var (
	HttpListenAndServe = http.ListenAndServe
	NetListen          = net.Listen
	ExitFn             = os.Exit

	RegisterScrapperGateway = pbv1.RegisterScrapperHandlerFromEndpoint
	NewGrpcServer           = grpc.NewServer
)

func New(server pbv1.ScrapperServer, log *zap.Logger, cfg *config.ScrapperConfig) Scrapper {
	return Scrapper{
		server: server,
		log:    log,
		cfg:    cfg,
	}
}

// Run - run servers for handle requests from tg bot
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

	address := s.cfg.ScrapperAddrGRPC
	err := RegisterScrapperGateway(ctx, mux, address, opts)
	if err != nil {
		s.log.Error("can not register grpc gateway", zap.Error(err))
		ExitFn(-1)
	}

	gatewayPort := s.cfg.ScrapperAddrHTTP
	s.log.Info("gateway listening at port", zap.String("port", gatewayPort))

	if err = HttpListenAndServe(gatewayPort, mux); err != nil {
		s.log.Error("gateway listen error", zap.Error(err))
	}
}

func (s *Scrapper) runGrpc() {
	port := s.cfg.ScrapperAddrGRPC
	lis, err := NetListen("tcp", port)
	if err != nil {
		s.log.Error("can open tcp socker", zap.Error(err))
		ExitFn(-1)
	}
	srv := NewGrpcServer()
	reflection.Register(srv)
	pbv1.RegisterScrapperServer(srv, s.server)

	s.log.Info("grpc server listening at port", zap.String("port", port))
	if err = srv.Serve(lis); err != nil {
		s.log.Error("grpc server listen error", zap.Error(err))
	}
}
