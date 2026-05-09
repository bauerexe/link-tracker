package clients

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

func TestDoWithResilience_RetryableStatus(t *testing.T) {
	var calls int32
	cfg := ResilienceConfig{RetryMaxAttempts: 3, RetryDelay: 20 * time.Millisecond, RetryableStatuses: map[int]struct{}{500: {}}}
	resp, err := doWithResilience(context.Background(), cfg, func(context.Context) (*http.Response, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("err"))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})
	if err != nil || resp.StatusCode != 200 || calls != 3 {
		t.Fatalf("err=%v code=%v calls=%d", err, resp.StatusCode, calls)
	}
}

func TestDoWithResilience_NoRetryOnNonRetryable(t *testing.T) {
	var calls int32
	cfg := ResilienceConfig{RetryMaxAttempts: 3, RetryDelay: 10 * time.Millisecond, RetryableStatuses: map[int]struct{}{500: {}}}
	resp, err := doWithResilience(context.Background(), cfg, func(context.Context) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader("bad"))}, nil
	})
	if err != nil || resp.StatusCode != 400 || calls != 1 {
		t.Fatalf("err=%v code=%v calls=%d", err, resp.StatusCode, calls)
	}
}

func TestDoWithResilience_CircuitOpen(t *testing.T) {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "t", ReadyToTrip: func(c gobreaker.Counts) bool { return c.TotalFailures >= 1 }, Timeout: 200 * time.Millisecond})
	cfg := ResilienceConfig{RetryMaxAttempts: 1, CircuitBreaker: cb}
	_, _ = doWithResilience(context.Background(), cfg, func(context.Context) (*http.Response, error) { return nil, context.DeadlineExceeded })
	start := time.Now()
	_, err := doWithResilience(context.Background(), cfg, func(context.Context) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})
	if err == nil || !errors.Is(err, ErrCircuitBreakerOpen) {
		t.Fatalf("expected open err, got %v", err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatalf("expected fast fail")
	}
}

func TestGitHubClient_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"full_name":"a/b"}`))
	}))
	defer srv.Close()

	hc := &http.Client{Timeout: 50 * time.Millisecond}
	cl := &GitHubClient{httpClient: hc, log: zap.NewNop(), resilience: ResilienceConfig{RetryMaxAttempts: 1}}
	start := time.Now()
	_, err := cl.doRequest(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected timeout")
	}
	if time.Since(start) >= 200*time.Millisecond {
		t.Fatal("request waited too long")
	}
}
