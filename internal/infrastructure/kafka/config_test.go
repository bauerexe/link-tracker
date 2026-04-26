package kafka

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSaramaConfig(t *testing.T) {
	t.Parallel()

	cfg := New()

	require.NotNil(t, cfg)

	assert.Equal(t, sarama.V3_9_0_0, cfg.Version)
	assert.True(t, cfg.Producer.Return.Successes)
	assert.Equal(t, sarama.WaitForAll, cfg.Producer.RequiredAcks)
	assert.Equal(t, sarama.CompressionGZIP, cfg.Producer.Compression)
	assert.Equal(t, sarama.OffsetOldest, cfg.Consumer.Offsets.Initial)
	assert.False(t, cfg.Consumer.Offsets.AutoCommit.Enable)
	assert.Equal(t, time.Second, cfg.Consumer.Offsets.AutoCommit.Interval)
}

func TestNewSaramaConfig_WithOption(t *testing.T) {
	t.Parallel()

	cfg := New(func(c *sarama.Config) {
		c.ClientID = "test-client"
		c.Consumer.Offsets.Initial = sarama.OffsetNewest
	})

	assert.Equal(t, "test-client", cfg.ClientID)
	assert.Equal(t, sarama.OffsetNewest, cfg.Consumer.Offsets.Initial)
}
