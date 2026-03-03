package botapp

import (
	"context"
	"errors"
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

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/config"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

// Bot - use case
type Bot struct {
	botRepository BotGateway
	server        pbv1.BotServer
	router        *BotDispatcher
	log           *zap.Logger
	cfg           *config.BotConfig
}

const timeoutSec = 60

func NewBot(token string, botRepository BotGateway, server pbv1.BotServer, router *BotDispatcher, log *zap.Logger, cfg *config.BotConfig) (*Bot, error) {
	log = log.Named("application")
	log = log.With(zap.String("pkg", "botapp"))

	if server == nil {
		log.Warn("grps server is nil")
	}

	if token == "" {
		log.Error("error empty telegram token")
		return nil, errors.New("empty telegram token")
	}
	if router == nil {
		log.Error("error nil router")
		return nil, errors.New("nil router")
	}

	return &Bot{botRepository: botRepository, server: server, router: router, log: log, cfg: cfg}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	updates, err := b.botRepository.GetMessages(ctx, timeoutSec)
	if err != nil {
		b.log.Error("failed to get messages", zap.Error(err))
		return err
	}

	b.log.Info("bot run")
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGILL, syscall.SIGTERM)
	defer cancel()

	if b.server != nil {
		go b.runGrpc()
		go b.runRest(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			b.log.Info("ctx done", zap.Error(ctx.Err()))
			time.Sleep(time.Second * 3)
			return ctx.Err()

		case upd, ok := <-updates:
			cont, err := b.handleIncomingMessage(upd, ok)
			if err != nil && !errors.Is(err, ErrorUnknownCommand) {
				return err
			}
			if !cont {
				return nil
			}
		}
	}
}

func (b *Bot) handleIncomingMessage(upd domain.Message, ok bool) (bool, error) {
	if !ok {
		b.log.Info("updates channel closed")
		return false, nil
	}

	logger := b.log.With(
		zap.String("msg", upd.Text),
		zap.Int("msgId", upd.MessageID),
		zap.Int64("chatId", upd.ChatID),
	)
	logger.Info("bot got message")

	text := strings.TrimSpace(upd.Text)
	if text == "" {
		return true, ErrorUnknownCommand
	}

	if err := b.processMessage(logger, upd, text); err != nil {
		return false, err
	}

	return true, nil
}

func (b *Bot) processMessage(logger *zap.Logger, upd domain.Message, text string) error {
	cmd, args := parseCommand(text)

	replyText, err := b.router.Dispatch(upd, domain.Command(cmd), args)
	if err != nil {
		logger.Error(
			"error invalid command",
			zap.String("command", cmd),
			zap.Error(err),
		)
		return err
	}

	logger.Info("bot dispatched message to reply", zap.String("reply", replyText))

	if err := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, replyText); err != nil {
		logger.Error("error send message", zap.Error(err))
		return err
	}

	logger.Info("bot sent reply message to user", zap.String("reply", replyText))
	return nil
}

func parseCommand(text string) (cmd, args string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", ""
	}
	cmd = parts[0]
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}
	return cmd, args
}

func (b *Bot) runRest(ctx context.Context) {
	mux := grpcruntime.NewServeMux(
		grpcruntime.WithIncomingHeaderMatcher(func(k string) (string, bool) {
			if strings.EqualFold(k, "Tg-Chat-Id") {
				return "tg-chat-id", true
			}
			return grpcruntime.DefaultHeaderMatcher(k)
		}))
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	address := b.cfg.BotAddrGRPC
	err := pbv1.RegisterBotHandlerFromEndpoint(ctx, mux, address, opts)
	if err != nil {
		b.log.Error("can not register grpc gateway", zap.Error(err))
		os.Exit(-1)
	}

	gatewayPort := b.cfg.BotAddrHTTP
	b.log.Info("gateway listening at port", zap.String("port", gatewayPort))

	if err = http.ListenAndServe(gatewayPort, mux); err != nil {
		b.log.Error("gateway listen error", zap.Error(err))
	}
}

func (b *Bot) runGrpc() {
	port := b.cfg.BotAddrGRPC
	lis, err := net.Listen("tcp", port)
	if err != nil {
		b.log.Error("can open tcp socker", zap.Error(err))
		os.Exit(-1)
	}
	srv := grpc.NewServer()
	reflection.Register(srv)
	pbv1.RegisterBotServer(srv, b.server)

	b.log.Info("grpc server listening at port", zap.String("port", port))
	if err = srv.Serve(lis); err != nil {
		b.log.Error("grpc server listen error", zap.Error(err))
	}
}
