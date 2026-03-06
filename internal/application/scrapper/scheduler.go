package scrapper_app

import (
	"context"
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type Scheduler struct {
	Links    LinkRepository
	Notifier BotNotifier
	Checkers []Checker
	Log      *zap.Logger

	Interval time.Duration
}

func NewScheduler(scheduler *Scheduler) (*Scheduler, error) {
	scheduler.Log = scheduler.Log.Named("application")
	scheduler.Log = scheduler.Log.With(zap.String("pkg", "scrapper"))
	if scheduler.Interval == 0 {
		scheduler.Interval = 2 * time.Minute
	}
	if scheduler.Links == nil {
		scheduler.Log.Error("err: nil links")
		return nil, fmt.Errorf("err: nil links")
	}
	if scheduler.Notifier == nil {
		scheduler.Log.Error("err: nil notifier")
		return nil, fmt.Errorf("err: nil notifier")
	}
	if len(scheduler.Checkers) == 0 || scheduler.Checkers == nil {
		scheduler.Log.Error("err: nil checkers or zero elements in checkers")
		return nil, fmt.Errorf("err: nil checkers or zero elements in checkers")
	}
	return scheduler, nil
}

func (s *Scheduler) Run(ctx context.Context) {
	sch, err := gocron.NewScheduler()
	if err != nil {
		s.Log.Error("scheduler init failed", zap.Error(err))
		return
	}
	defer func() { _ = sch.Shutdown() }()

	task := func() {
		now := time.Now()
		s.tickWithNow(ctx, now)
	}

	job, err := sch.NewJob(
		gocron.DurationJob(s.Interval),
		gocron.NewTask(task),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		s.Log.Error("schedule job failed", zap.Error(err))
		return
	}

	go func() { _ = job.RunNow() }()

	sch.Start()

	<-ctx.Done()
	_ = sch.Shutdown()
}

func (s *Scheduler) tickWithNow(ctx context.Context, now time.Time) {
	jobCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	links, err := s.Links.ListLinks(jobCtx)
	if err != nil {
		s.Log.Error("list links failed", zap.Error(err))
		return
	}

	for _, l := range links {
		s.processURL(jobCtx, now, l.URL)
	}
}

func (s *Scheduler) processURL(ctx context.Context, now time.Time, url string) {
	checker := s.pick(url)
	if checker == nil {
		s.Log.Debug("no checker for url", zap.String("url", url))
		return
	}
	s.Log.Debug("picked checker", zap.String("url", url), zap.String("checker", fmt.Sprintf("%T", checker)))

	st, since, ok := s.loadState(ctx, url)
	if !ok {
		return
	}

	desc, updatedAt, updated, err := checker.Check(ctx, url, since)
	if err != nil {
		s.Log.Error("check failed", zap.String("url", url), zap.Error(err))
		s.saveChecked(ctx, url, st, now)
		return
	}

	st.LastCheckedAt = now

	if !updated {
		s.saveState(ctx, url, st)
		return
	}

	chatIDs, ok := s.loadChatIDs(ctx, url, st)
	if !ok {
		return
	}

	if !s.notify(ctx, url, desc, chatIDs, st) {
		return
	}

	s.updateLastUpdatedAt(&st, updatedAt)
	s.saveState(ctx, url, st)
}

func (s *Scheduler) loadState(ctx context.Context, url string) (domain.URLState, time.Time, bool) {
	st, err := s.Links.GetURLState(ctx, url)
	if err != nil {
		s.Log.Error("get url state failed", zap.String("url", url), zap.Error(err))
		return domain.URLState{}, time.Time{}, false
	}
	return st, st.LastUpdatedAt, true
}

func (s *Scheduler) saveChecked(ctx context.Context, url string, st domain.URLState, now time.Time) {
	st.LastCheckedAt = now
	_ = s.Links.SetURLState(ctx, url, st)
}

func (s *Scheduler) loadChatIDs(ctx context.Context, url string, st domain.URLState) ([]int64, bool) {
	chatIDs, err := s.Links.GetChatIDsByLink(ctx, url)
	if err != nil {
		s.Log.Error("get chat ids failed", zap.String("url", url), zap.Error(err))
		s.saveState(ctx, url, st)
		return nil, false
	}
	return chatIDs, true
}

func (s *Scheduler) notify(ctx context.Context, url, desc string, chatIDs []int64, st domain.URLState) bool {
	if err := s.Notifier.Notify(ctx, url, desc, chatIDs); err != nil {
		s.Log.Error("notify bot failed", zap.String("url", url), zap.Error(err))
		s.saveState(ctx, url, st)
		return false
	}
	return true
}

func (s *Scheduler) updateLastUpdatedAt(st *domain.URLState, updatedAt time.Time) {
	if updatedAt.After(st.LastUpdatedAt) || st.LastUpdatedAt.IsZero() {
		st.LastUpdatedAt = updatedAt
	}
}

func (s *Scheduler) saveState(ctx context.Context, url string, st domain.URLState) {
	if err := s.Links.SetURLState(ctx, url, st); err != nil {
		s.Log.Error("set url state failed", zap.String("url", url), zap.Error(err))
	}
}

func (s *Scheduler) pick(url string) Checker {
	for _, c := range s.Checkers {
		if c.Match(url) {
			return c
		}
	}
	return nil
}
