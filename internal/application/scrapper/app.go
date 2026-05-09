package scrapperapp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	grpcruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*rate.Limiter
	r        rate.Limit
	b        int
}

func newIPRateLimiter(r float64, b int) *ipRateLimiter {
	return &ipRateLimiter{visitors: map[string]*rate.Limiter{}, r: rate.Limit(r), b: b}
}
func (l *ipRateLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	if v, ok := l.visitors[ip]; ok {
		return v
	}
	nl := rate.NewLimiter(l.r, l.b)
	l.visitors[ip] = nl
	return nl
}

// Scrapper - use case for service to keep info for scraping links
type Scrapper struct {
	server pbv1.ScrapperServer
	log    *zap.Logger
	cfg    *config.ScrapperConfig

	mu         sync.Mutex
	wg         sync.WaitGroup
	grpcServer *grpc.Server
}

var (
	NetListen = net.Listen
	ExitFn    = os.Exit
	HTTPServe = http.Serve

	RegisterScrapperGateway = pbv1.RegisterScrapperHandlerFromEndpoint
	NewGrpcServer           = grpc.NewServer
)

const (
	shutdownTimeout     = 5 * time.Second
	gracefulStopTimeout = 5 * time.Second
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
	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s.wg.Go(func() { s.runGrpc() })
	s.wg.Go(func() { s.runRest(sigCtx) })

	<-sigCtx.Done()
	stop()
	s.log.Info("shutdown signal received", zap.Error(sigCtx.Err()))
	s.shutdown()
	if err := s.waitWithTimeout(gracefulStopTimeout); err != nil {
		s.log.Warn("scrapper shutdown finished with timeout", zap.Error(err))
	}
}

func (s *Scrapper) runRest(ctx context.Context) {
	mux := grpcruntime.NewServeMux(
		grpcruntime.WithIncomingHeaderMatcher(func(k string) (string, bool) {
			if strings.EqualFold(k, "Tg-Chat-Id") {
				return "tg-chat-id", true
			}
			return grpcruntime.DefaultHeaderMatcher(k)
		}),
	)

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	address := s.cfg.ScrapperAddrGRPC
	if err := RegisterScrapperGateway(ctx, mux, address, opts); err != nil {
		s.log.Error("can not register grpc gateway", zap.Error(err))
		return
	}

	ln, err := NetListen("tcp", s.cfg.ScrapperAddrHTTP)
	if err != nil {
		s.log.Error("gateway listen error", zap.Error(err))
		ExitFn(-1)
		return
	}

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	h := http.Handler(mux)
	if s.cfg.RateLimitRPS > 0 && s.cfg.RateLimitBurst > 0 {
		rl := newIPRateLimiter(s.cfg.RateLimitRPS, s.cfg.RateLimitBurst)
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip == "" {
				ip = r.RemoteAddr
			}
			if !rl.get(ip).Allow() {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			mux.ServeHTTP(w, r)
		})
	}
	s.log.Info("gateway listening at port", zap.String("port", s.cfg.ScrapperAddrHTTP))

	if err = HTTPServe(ln, h); err != nil && !errors.Is(err, net.ErrClosed) {
		s.log.Error("gateway serve error", zap.Error(err))
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
	s.grpcServer = srv

	reflection.Register(srv)
	pbv1.RegisterScrapperServer(srv, s.server)

	s.log.Info("grpc server listening at port", zap.String("port", port))
	if err = srv.Serve(lis); err != nil {
		s.log.Error("grpc server listen error", zap.Error(err))
	}
}

func (s *Scrapper) waitWithTimeout(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}

func (s *Scrapper) shutdown() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	s.mu.Lock()
	grpcServer := s.grpcServer
	s.mu.Unlock()

	if grpcServer != nil {
		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
		case <-shutdownCtx.Done():
			s.log.Warn("grpc graceful stop timeout, forcing stop", zap.Error(shutdownCtx.Err()))
			grpcServer.Stop()
			<-done
		}
	}
}
