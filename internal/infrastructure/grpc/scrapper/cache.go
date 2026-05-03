package scrappercontroller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type Cache interface {
	GetLinks(ctx context.Context, chatID int64) (*pbv1.ListLinksResponse, bool, error)
	SetLinks(ctx context.Context, chatID int64, resp *pbv1.ListLinksResponse) error
	InvalidateLinks(ctx context.Context, chatID int64) error
}

type noopCache struct{}

func (n noopCache) GetLinks(context.Context, int64) (*pbv1.ListLinksResponse, bool, error) {
	return nil, false, nil
}

func (n noopCache) SetLinks(context.Context, int64, *pbv1.ListLinksResponse) error {
	return nil
}

func (n noopCache) InvalidateLinks(context.Context, int64) error {
	return nil
}

type valkeyCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewValkeyCache(client *redis.Client, ttl time.Duration) Cache {
	if client == nil || ttl <= 0 {
		return noopCache{}
	}

	return &valkeyCache{
		client: client,
		ttl:    ttl,
	}
}

func (c *valkeyCache) GetLinks(
	ctx context.Context,
	chatID int64,
) (*pbv1.ListLinksResponse, bool, error) {
	val, err := c.client.Get(ctx, c.key(chatID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("get links cache: %w", err)
	}

	var resp pbv1.ListLinksResponse
	if err = json.Unmarshal([]byte(val), &resp); err != nil {
		return nil, false, fmt.Errorf("unmarshal links cache: %w", err)
	}

	return &resp, true, nil
}

func (c *valkeyCache) SetLinks(
	ctx context.Context,
	chatID int64,
	resp *pbv1.ListLinksResponse,
) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal links cache: %w", err)
	}

	if err = c.client.Set(ctx, c.key(chatID), b, c.ttl).Err(); err != nil {
		return fmt.Errorf("set links cache: %w", err)
	}

	return nil
}

func (c *valkeyCache) InvalidateLinks(ctx context.Context, chatID int64) error {
	if err := c.client.Del(ctx, c.key(chatID)).Err(); err != nil {
		return fmt.Errorf("invalidate links cache: %w", err)
	}

	return nil
}

func (c *valkeyCache) key(chatID int64) string {
	return fmt.Sprintf("links:%d", chatID)
}
