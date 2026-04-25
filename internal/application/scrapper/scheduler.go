package scrapperapp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

const (
	defaultInterval    = 2 * time.Minute
	contextTimeout     = 2 * time.Minute
	saveCtxTimeout     = 5 * time.Second
	sendCtxTimeout     = 10 * time.Second
	defaultBatchSize   = 100
	defaultWorkerCount = 4

	minBatchSize = 50
	maxBatchSize = 500
)

type FailedLink struct {
	URL   string
	Error string
}

type WorkerPool struct {
	Jobs chan func()
	Log  *zap.Logger
}

func NewWorkerPool(workers, queueSize int, log *zap.Logger) (*WorkerPool, error) {
	if workers < 1 {
		return nil, errors.New("err: worker count must be greater than zero")
	}
	if queueSize < 1 {
		return nil, errors.New("err: queue size must be greater than zero")
	}

	return &WorkerPool{
		Jobs: make(chan func(), queueSize),
		Log:  log,
	}, nil
}

func (w *WorkerPool) Run(ctx context.Context, workers int) {
	for range workers {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-w.Jobs:
					if !ok {
						return
					}
					if job != nil {
						job()
					}
				}
			}
		}()
	}
}

type Scheduler struct {
	Links    LinkRepository
	Notifier BotNotifier
	Checkers []Checker
	Log      *zap.Logger

	Interval               time.Duration
	BatchSize, WorkerCount int

	mu              sync.RWMutex
	lastFailedLinks []FailedLink
	pool            *WorkerPool
}

func NewScheduler(s *Scheduler) (*Scheduler, error) {
	if s.Log == nil {
		return nil, errors.New("err: nil logger")
	}
	s.Log = s.Log.Named("application").With(zap.String("pkg", "scrapper"))

	if s.Interval == 0 {
		s.Interval = defaultInterval
	}
	if s.BatchSize == 0 {
		s.BatchSize = defaultBatchSize
	}
	if s.WorkerCount == 0 {
		s.WorkerCount = defaultWorkerCount
	}

	switch {
	case s.BatchSize < minBatchSize || s.BatchSize > maxBatchSize:
		return nil, fmt.Errorf("err: batch size must be in range [%d;%d]", minBatchSize, maxBatchSize)
	case s.WorkerCount < 1:
		return nil, errors.New("err: worker count must be greater than zero")
	case s.Links == nil:
		return nil, errors.New("err: nil links")
	case s.Notifier == nil:
		return nil, errors.New("err: nil notifier")
	case len(s.Checkers) == 0:
		return nil, errors.New("err: nil clients or zero elements in clients")
	}

	queueSize := s.BatchSize * s.WorkerCount
	pool, err := NewWorkerPool(s.WorkerCount, queueSize, s.Log)
	if err != nil {
		return nil, err
	}
	s.pool = pool

	return s, nil
}

func (s *Scheduler) Run(ctx context.Context) {
	s.pool.Run(ctx, s.WorkerCount)
	defer close(s.pool.Jobs)

	sch, err := gocron.NewScheduler()
	if err != nil {
		s.Log.Error("scheduler init failed", zap.Error(err))
		return
	}
	defer func() { _ = sch.Shutdown() }()

	job, err := sch.NewJob(
		gocron.DurationJob(s.Interval),
		gocron.NewTask(func() { s.tick(ctx, time.Now().UTC()) }),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		s.Log.Error("schedule job failed", zap.Error(err))
		return
	}

	go func() { _ = job.RunNow() }()
	sch.Start()

	<-ctx.Done()
}

func (s *Scheduler) LastFailedLinks() []FailedLink {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]FailedLink(nil), s.lastFailedLinks...)
}

func (s *Scheduler) tick(parentCtx context.Context, now time.Time) {
	ctx, cancel := context.WithTimeout(parentCtx, contextTimeout)
	defer cancel()

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed []FailedLink
	)

	s.loadBatches(ctx, now, &wg, &mu, &failed)

	wg.Wait()
	s.setLastFailedLinks(failed)

	if len(failed) > 0 {
		s.notifyFailedLinks(ctx, failed)
		s.Log.Warn("scheduler run finished with failed links",
			zap.Int("failed_count", len(failed)),
			zap.Any("failed_links", failed),
		)
		return
	}

	if ctx.Err() != nil {
		s.Log.Warn("scheduler run finished by timeout/cancel", zap.Error(ctx.Err()))
		return
	}

	s.Log.Info("scheduler run finished successfully")
}

func (s *Scheduler) loadBatches(
	ctx context.Context,
	now time.Time,
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	failed *[]FailedLink,
) {
	for offset := 0; ; offset += s.BatchSize {
		links, err := s.Links.ListLinksBatch(ctx, s.BatchSize, offset)
		if err != nil {
			s.Log.Error("list links batch failed", zap.Error(err))
			return
		}

		if len(links) == 0 {
			return
		}

		for _, link := range links {
			if link == nil || link.URL == "" {
				continue
			}

			wg.Add(1)
			select {
			case <-ctx.Done():
				wg.Done()
				return
			case s.pool.Jobs <- s.createLinkTask(ctx, now, link.URL, wg, mu, failed):
			}
		}
	}
}

func (s *Scheduler) createLinkTask(
	ctx context.Context,
	now time.Time,
	url string,
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	failed *[]FailedLink,
) func() {
	return func() {
		defer wg.Done()

		if procErr := s.processURL(ctx, now, url); procErr != nil {
			mu.Lock()
			*failed = append(*failed, FailedLink{
				URL:   url,
				Error: procErr.Error(),
			})
			mu.Unlock()
		}
	}
}

func (s *Scheduler) processURL(ctx context.Context, now time.Time, url string) error {
	checker := s.pick(url)
	if checker == nil {
		s.Log.Debug("no checker for url", zap.String("url", url))
		return fmt.Errorf("no checker for url: %s", url)
	}

	state, err := s.Links.GetURLState(ctx, url)
	if err != nil {
		s.Log.Error("get url state failed", zap.String("url", url), zap.Error(err))
		return fmt.Errorf("get url state failed: %w", err)
	}

	state.LastCheckedAt = now

	fail := func(msg string, err error) error {
		s.Log.Error(msg, zap.String("url", url), zap.Error(err))

		saveCtx, cancel := context.WithTimeout(context.Background(), saveCtxTimeout)
		defer cancel()
		_ = s.saveState(saveCtx, url, state)

		return fmt.Errorf("%s: %w", msg, err)
	}

	desc, updatedAt, updated, err := checker.Check(ctx, url, state.LastUpdatedAt)
	if err != nil {
		return fail("check failed", err)
	}

	if state.LastUpdatedAt.IsZero() {
		if updatedAt.IsZero() {
			updatedAt = now
		}
		state.LastUpdatedAt = updatedAt
		return s.saveState(ctx, url, state)
	}

	if !updated {
		return s.saveState(ctx, url, state)
	}

	chatIDs, err := s.Links.GetChatIDsByLink(ctx, url)
	if err != nil {
		return fail("get chat ids failed", err)
	}

	if err = s.Notifier.Notify(ctx, url, desc, chatIDs); err != nil {
		return fail("notify bot failed", err)
	}

	if updatedAt.After(state.LastUpdatedAt) {
		state.LastUpdatedAt = updatedAt
	}

	return s.saveState(ctx, url, state)
}

func (s *Scheduler) saveState(ctx context.Context, url string, state domain.URLState) error {
	if err := s.Links.SetURLState(ctx, url, state); err != nil {
		s.Log.Error("set url state failed", zap.String("url", url), zap.Error(err))
		return fmt.Errorf("set url state failed: %w", err)
	}
	return nil
}

func (s *Scheduler) pick(url string) Checker {
	for _, c := range s.Checkers {
		if c.Match(url) {
			return c
		}
	}
	return nil
}

func (s *Scheduler) setLastFailedLinks(items []FailedLink) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastFailedLinks = append([]FailedLink(nil), items...)
}

func (s *Scheduler) notifyFailedLinks(parentCtx context.Context, failed []FailedLink) {
	if len(failed) == 0 {
		return
	}

	for _, item := range failed {
		chatIDs, err := s.Links.GetChatIDsByLink(parentCtx, item.URL)
		if err != nil {
			s.Log.Error("get chat ids by failed link failed",
				zap.String("url", item.URL),
				zap.Error(err),
			)
			continue
		}
		if len(chatIDs) == 0 {
			s.Log.Warn("no chat ids for failed link report",
				zap.String("url", item.URL),
			)
			continue
		}

		msg := fmt.Sprintf(
			"Не удалось обработать ссылку:\n%s\n\nОшибка: %s",
			item.URL,
			item.Error,
		)

		sendCtx, cancel := context.WithTimeout(context.Background(), sendCtxTimeout)
		err = s.Notifier.Notify(
			sendCtx,
			"failed-links-report",
			msg,
			chatIDs,
		)
		cancel()

		if err != nil {
			s.Log.Error("notify failed link report failed",
				zap.String("url", item.URL),
				zap.Error(err),
			)
		}
	}
}
