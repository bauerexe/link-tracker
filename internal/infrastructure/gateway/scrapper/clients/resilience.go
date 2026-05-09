package clients

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/avast/retry-go"
	"github.com/sony/gobreaker"
)

type ResilienceConfig struct {
	RetryMaxAttempts  uint
	RetryDelay        time.Duration
	RetryableStatuses map[int]struct{}
	CircuitBreaker    *gobreaker.CircuitBreaker
}

var ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

func doWithResilience(ctx context.Context, cfg ResilienceConfig, call func(context.Context) (*http.Response, error)) (*http.Response, error) {
	exec := func() (*http.Response, error) {
		if cfg.CircuitBreaker == nil {
			return call(ctx)
		}
		out, err := cfg.CircuitBreaker.Execute(func() (interface{}, error) {
			return call(ctx) //nolint:bodyclose // линтер
		})
		if err != nil {
			if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
				return nil, ErrCircuitBreakerOpen
			}
			return nil, fmt.Errorf("resilience %w", err)
		}
		resp, ok := out.(*http.Response)
		if !ok {
			return nil, fmt.Errorf("unexpected response type: %T", out)
		}
		return resp, nil
	}
	if cfg.RetryMaxAttempts <= 1 {
		return exec()
	}
	var response *http.Response
	err := retry.Do(
		func() error {
			resp, err := exec()
			if err != nil {
				return err
			}
			if _, ok := cfg.RetryableStatuses[resp.StatusCode]; ok {
				_ = resp.Body.Close()
				return fmt.Errorf("retryable status: %d", resp.StatusCode)
			}
			response = resp
			return nil
		},
		retry.Context(ctx),
		retry.Attempts(cfg.RetryMaxAttempts),
		retry.Delay(cfg.RetryDelay),
		retry.DelayType(func(_ uint, _ error, _ *retry.Config) time.Duration { return cfg.RetryDelay }),
	)
	if err != nil {
		return nil, fmt.Errorf("resilience %w", err)
	}
	return response, nil
}
