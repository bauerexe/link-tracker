package botapp

import (
	"context"
	"errors"
	"fmt"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

// Bot - use case service of tg bot for track links
type Bot struct {
	botRepository BotGateway
	server        pbv1.BotServer
	router        *BotDispatcher
	log           *zap.Logger
	cfg           *config.BotConfig
	fsm           *fsmStore

	mu         sync.Mutex
	wg         sync.WaitGroup
	grpcServer *grpc.Server
}

var (
	NetListen          = net.Listen
	ExitFn             = os.Exit
	HTTPServe          = http.Serve
	RegisterBotGateway = pbv1.RegisterBotHandlerFromEndpoint
	NewGrpcServer      = grpc.NewServer
)

var ErrEmptyText = errors.New("error empty text")

const (
	timeoutSec          = 60
	shutdownTimeout     = 5 * time.Second
	gracefulStopTimeout = 5 * time.Second
)

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

	return &Bot{botRepository: botRepository, server: server, router: router, log: log, cfg: cfg, fsm: newFSMStore()}, nil
}

// Run - run service tg bot and servers for handle requests from scrapper
func (b *Bot) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("run bot: context error: %w", err)
	}
	updates, err := b.botRepository.GetMessages(ctx, timeoutSec)
	if err != nil {
		b.log.Error("failed to get messages", zap.Error(err))
		return fmt.Errorf("run bot: get messages: %w", err)
	}

	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	b.log.Info("bot run")

	if b.server != nil {
		b.wg.Go(func() { b.runGrpc() })
		b.wg.Go(func() { b.runRest(sigCtx) })
	}

	for {
		select {
		case <-sigCtx.Done():
			stop()
			b.log.Info("shutdown signal received", zap.Error(sigCtx.Err()))
			b.shutdown()
			if err = b.waitWithTimeout(gracefulStopTimeout); err != nil {
				b.log.Warn("bot shutdown finished with timeout", zap.Error(err))
			}
			return fmt.Errorf("run bot: shutdown signal: %w", sigCtx.Err())
		case upd, ok := <-updates:
			var cont bool
			cont, err = b.handleIncomingMessage(upd, ok)
			if err != nil && !errors.Is(err, ErrUnknownCommand) {
				b.log.Error("failed to handle incoming message", zap.Error(err))
			}
			if !cont {
				b.log.Error("failed to handle incoming message")
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
		return true, ErrEmptyText
	}

	handled, err := b.handleTrackDialog(logger, upd)
	if err != nil {
		return false, err
	}
	if handled {
		return true, nil
	}

	if err = b.processMessage(logger, upd); err != nil {
		return false, err
	}

	return true, nil
}

func (b *Bot) processMessage(logger *zap.Logger, upd domain.Message) error {
	cmd, args := upd.Command, upd.Arguments

	replyText, err := b.router.Dispatch(upd, cmd, args)
	if err != nil {
		if errors.Is(err, ErrUnknownCommand) {
			replyText = "Не знаю команду " + cmd.String() + ". Напиши /help"
		} else {
			logger.Error(
				"error invalid command",
				zap.String("command", cmd.String()),
				zap.Error(err),
			)
			return err
		}
	}

	logger.Info("bot dispatched message to reply", zap.String("reply", replyText))

	sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, replyText)
	if sendErr != nil {
		logger.Error("error send message", zap.Error(sendErr))
		return fmt.Errorf("process message: send reply: %w", sendErr)
	}

	logger.Info("bot sent reply message to user", zap.String("reply", replyText))
	return nil
}

func (b *Bot) runRest(ctx context.Context) {
	mux := grpcruntime.NewServeMux(
		grpcruntime.WithIncomingHeaderMatcher(func(k string) (string, bool) {
			if strings.EqualFold(k, "Tg-Chat-Id") {
				return "tg-chat-id", true
			}
			return grpcruntime.DefaultHeaderMatcher(k)
		}),
	)

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	address := b.cfg.BotAddrGRPC
	if err := RegisterBotGateway(ctx, mux, address, opts); err != nil {
		b.log.Error("can not register grpc gateway", zap.Error(err))
		return
	}

	ln, err := NetListen("tcp", b.cfg.BotAddrHTTP)
	if err != nil {
		b.log.Error("gateway listen error", zap.Error(err))
		ExitFn(-1)
		return
	}

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	b.log.Info("gateway listening at port", zap.String("port", b.cfg.BotAddrHTTP))

	if err = HTTPServe(ln, mux); err != nil && !errors.Is(err, net.ErrClosed) {
		b.log.Error("gateway serve error", zap.Error(err))
	}
}

func (b *Bot) runGrpc() {
	port := b.cfg.BotAddrGRPC
	lis, err := NetListen("tcp", port)
	if err != nil {
		b.log.Error("can open tcp socker", zap.Error(err))
		ExitFn(-1)
	}
	srv := NewGrpcServer()
	b.mu.Lock()
	b.grpcServer = srv
	b.mu.Unlock()
	reflection.Register(srv)
	pbv1.RegisterBotServer(srv, b.server)

	b.log.Info("grpc server listening at port", zap.String("port", port))

	if err = srv.Serve(lis); err != nil {
		b.log.Error("grpc server listen error", zap.Error(err))
	}
}

func (b *Bot) handleTrackDialog(logger *zap.Logger, upd domain.Message) (bool, error) {
	if upd.Command.String() == "track" {
		return b.startTrackDialog(logger, upd)
	}
	logger.Info("handle track dialog", zap.String("command", upd.Command.String()), zap.String("arguments", upd.Arguments), zap.String("command", upd.Command.String()))
	b.fsm.mu.Lock()
	st := b.fsm.get(upd.ChatID)
	step := st.step
	b.fsm.mu.Unlock()

	if step == trackIdle {
		return false, nil
	}

	handled, err := b.handleTrackControlCommands(logger, upd)
	if handled || err != nil {
		return handled, err
	}

	switch step {
	case trackIdle:
		return false, nil

	case trackWaitURL:
		return b.handleTrackWaitURL(logger, upd)

	case trackWaitTags:
		return b.handleTrackWaitTags(logger, upd)

	default:
		b.fsm.mu.Lock()
		b.fsm.reset(upd.ChatID)
		b.fsm.mu.Unlock()
		return false, nil
	}
}

func (b *Bot) startTrackDialog(logger *zap.Logger, upd domain.Message) (bool, error) {
	b.fsm.mu.Lock()
	st := b.fsm.get(upd.ChatID)
	st.step = trackWaitURL
	st.draft = trackDraft{}
	b.fsm.mu.Unlock()

	reply := "Пришли ссылку для отслеживания. /cancel чтобы отменить."
	logger.Info("track dialog started")
	sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, reply)
	if sendErr != nil {
		return true, fmt.Errorf("start track dialog: send message: %w", sendErr)
	}

	return true, nil
}

func (b *Bot) handleTrackControlCommands(logger *zap.Logger, upd domain.Message) (bool, error) {
	if upd.Command == "skip" {
		b.fsm.mu.Lock()
		url := b.fsm.get(upd.ChatID).draft.url
		b.fsm.reset(upd.ChatID)
		b.fsm.mu.Unlock()

		replyText, err := b.router.Dispatch(upd, "track", url)
		if err != nil {
			logger.Error("track dispatch failed", zap.Error(err))
			sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, err.Error())
			if sendErr != nil {
				return true, fmt.Errorf("track control skip: send error message: %w", sendErr)
			}

			return true, nil
		}

		logger.Info("track dialog completed", zap.String("url", url), zap.Int("tags_n", 0))
		sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, replyText)
		if sendErr != nil {
			return true, fmt.Errorf("track control skip: send reply: %w", sendErr)
		}

		return true, nil
	}

	if upd.Command == "cancel" {
		b.fsm.mu.Lock()
		b.fsm.reset(upd.ChatID)
		b.fsm.mu.Unlock()

		reply := "Ок, отменил."
		logger.Info("track dialog cancelled")
		sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, reply)
		if sendErr != nil {
			return true, fmt.Errorf("track control cancel: send reply: %w", sendErr)
		}

		return true, nil
	}

	if strings.HasPrefix(upd.Text, "/") {
		b.fsm.mu.Lock()
		b.fsm.reset(upd.ChatID)
		b.fsm.mu.Unlock()

		logger.Info("track dialog cancelled by another command", zap.String("cmd", upd.Text))
		return false, nil
	}

	return false, nil
}

func (b *Bot) handleTrackWaitURL(logger *zap.Logger, upd domain.Message) (bool, error) {
	url := strings.TrimSpace(upd.Text)
	if url == "" {
		reply := "Ссылка пустая. Пришли ссылку или /cancel."
		sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, reply)
		if sendErr != nil {
			return true, fmt.Errorf("track wait url: send empty-url reply: %w", sendErr)
		}

		return true, nil
	}
	if _, ok := handlers.NormalizeURL(url); !ok {
		reply := "Некорректная ссылка. Пример: https://example.com"
		sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, reply)
		if sendErr != nil {
			return true, fmt.Errorf("track wait url: send invalid-url reply: %w", sendErr)
		}

		return true, nil
	}

	b.fsm.mu.Lock()
	st := b.fsm.get(upd.ChatID)
	st.step = trackWaitTags
	st.draft.url = url
	b.fsm.mu.Unlock()

	reply := "Теперь теги (необязательно). Введи через запятую, или отправь /skip чтобы продолжить без тегов. /cancel чтобы отменить."
	logger.Info("track dialog got url", zap.String("url", url))
	sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, reply)
	if sendErr != nil {
		return true, fmt.Errorf("track wait url: send next-step reply: %w", sendErr)
	}

	return true, nil
}

func (b *Bot) handleTrackWaitTags(logger *zap.Logger, upd domain.Message) (bool, error) {
	tags := parseTagsCSV(upd.Text)

	b.fsm.mu.Lock()
	url := b.fsm.get(upd.ChatID).draft.url
	b.fsm.reset(upd.ChatID)
	b.fsm.mu.Unlock()

	args := url
	if len(tags) > 0 {
		args = url + " " + strings.Join(tags, " ")
	}

	replyText, err := b.router.Dispatch(upd, "track", args)
	if err != nil {
		logger.Error("track dispatch failed", zap.Error(err))
		sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, err.Error())
		if sendErr != nil {
			return true, fmt.Errorf("track wait tags: send error message: %w", sendErr)
		}

		return true, nil
	}

	logger.Info("track dialog completed", zap.String("url", url), zap.Int("tags_n", len(tags)))
	sendErr := b.botRepository.SendMessage(upd.ChatID, upd.MessageID, replyText)
	if sendErr != nil {
		return true, fmt.Errorf("track wait tags: send reply: %w", sendErr)
	}

	return true, nil
}

func parseTagsCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (b *Bot) waitWithTimeout(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}

func (b *Bot) shutdown() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	b.mu.Lock()
	grpcServer := b.grpcServer
	b.mu.Unlock()

	if grpcServer != nil {
		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
		case <-shutdownCtx.Done():
			b.log.Warn("grpc graceful stop timeout, forcing stop", zap.Error(shutdownCtx.Err()))
			grpcServer.Stop()
			<-done
		}
	}
}
