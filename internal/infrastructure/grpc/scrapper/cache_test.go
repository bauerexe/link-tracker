package scrappercontroller

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

func TestValkeyCache_SetGetInvalidateAndTTL(t *testing.T) {
	requireDocker(t)
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "valkey/valkey:8.1-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp"),
		},
	})
	require.NoError(t, err)
	defer c.Terminate(ctx)

	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "6379/tcp")
	client := redis.NewClient(&redis.Options{Addr: host + ":" + port.Port()})
	defer client.Close()

	cache := NewValkeyCache(client, 1*time.Second)
	in := &pbv1.ListLinksResponse{Links: []*pbv1.LinkResponse{{Id: 1, Url: "https://example.com"}}, Size: 1}
	require.NoError(t, cache.SetLinks(ctx, 10, in))

	out, ok, err := cache.GetLinks(ctx, 10)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, in.Size, out.Size)
	require.Equal(t, in.Links[0].Url, out.Links[0].Url)

	require.NoError(t, cache.InvalidateLinks(ctx, 10))
	_, ok, err = cache.GetLinks(ctx, 10)
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, cache.SetLinks(ctx, 11, in))
	time.Sleep(1200 * time.Millisecond)
	_, ok, err = cache.GetLinks(ctx, 11)
	require.NoError(t, err)
	require.False(t, ok)
}

func requireDocker(t *testing.T) {
	t.Helper()

	if os.Getenv("DOCKER_HOST") != "" {
		return
	}

	if runtime.GOOS == "windows" {
		if _, err := os.Stat(`\\.\pipe\docker_engine`); err == nil {
			return
		}

		t.Skip("docker is not available")
	}

	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skip("docker is not available")
	}
}
