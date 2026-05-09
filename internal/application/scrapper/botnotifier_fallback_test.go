package scrapperapp

import (
	"context"
	"errors"
	"testing"
)

type stubNotifier struct {
	err   error
	calls int
}

func (s *stubNotifier) Notify(context.Context, string, string, []int64) error {
	s.calls++
	return s.err
}

func TestFallbackBotNotifier_UsesFallbackOnPrimaryError(t *testing.T) {
	p := &stubNotifier{err: errors.New("down")}
	f := &stubNotifier{}
	n := NewFallbackBotNotifier(p, f, nil)
	if err := n.Notify(context.Background(), "u", "d", []int64{1}); err != nil {
		t.Fatal(err)
	}
	if p.calls != 1 || f.calls != 1 {
		t.Fatalf("calls p=%d f=%d", p.calls, f.calls)
	}
}
