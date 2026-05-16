package kafka

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agentapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/agent"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"go.uber.org/zap"
)

type fakeProducer struct {
	messages []*sarama.ProducerMessage
	err      error
}

func (f *fakeProducer) SendMessage(msg *sarama.ProducerMessage) (int32, int64, error) {
	f.messages = append(f.messages, msg)
	if f.err != nil {
		return 0, 0, f.err
	}

	return 0, 0, nil
}

func (f *fakeProducer) Close() error {
	return nil
}

type brokenProcessor struct{}

func (brokenProcessor) Process(_ context.Context, update agentapp.Update) (agentapp.Update, bool, error) {
	return update, false, errors.New("boom")
}

func newAgentTestHandler(t *testing.T, processor UpdateProcessor, producer messageProducer) *AgentHandler {
	t.Helper()

	codec, err := NewAvroCodec(testUpdateLinkSchema)
	require.NoError(t, err)

	return NewAgentHandler(zap.NewNop(), processor, producer, "link.processed-updates", "dead-letter-topic", codec)
}

func TestAgentHandlerConsumeClaim_ProducesProcessedMessage(t *testing.T) {
	t.Parallel()

	processor := agentapp.NewProcessor(config.AgentConfig{
		Summarization: config.AgentSummarizationConfig{Threshold: 10},
	}, agentapp.NewStubSummarizer())
	producer := &fakeProducer{}
	handler := newAgentTestHandler(t, processor, producer)

	req := UpdateLinkAvro{
		ID:          1,
		URL:         "https://example.com",
		Description: strings.Repeat("a", 20),
		TgChatIDs:   []int64{100},
	}

	data, err := handler.avro.Marshal(req)
	require.NoError(t, err)

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{Topic: "link.raw-updates", Value: data}
	close(messages)

	session := &fakeSession{}
	claim := &fakeClaim{messages: messages}

	err = handler.ConsumeClaim(session, claim)
	require.NoError(t, err)
	require.Len(t, producer.messages, 1)
	assert.Equal(t, "link.processed-updates", producer.messages[0].Topic)
	assert.Len(t, session.marked, 1)

	payload, err := producer.messages[0].Value.Encode()
	require.NoError(t, err)

	var actual UpdateLinkAvro
	err = handler.avro.Unmarshal(payload, &actual)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("a", 10)+"...", actual.Description)
}

func TestAgentHandlerConsumeClaim_FiltersMessage(t *testing.T) {
	t.Parallel()

	processor := agentapp.NewProcessor(config.AgentConfig{
		Filtering: config.AgentFilteringConfig{StopWords: []string{"spam"}},
	}, agentapp.NewStubSummarizer())
	producer := &fakeProducer{}
	handler := newAgentTestHandler(t, processor, producer)

	req := UpdateLinkAvro{
		ID:          2,
		URL:         "https://example.com",
		Description: "contains spam inside",
		TgChatIDs:   []int64{100},
	}

	data, err := handler.avro.Marshal(req)
	require.NoError(t, err)

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{Topic: "link.raw-updates", Value: data}
	close(messages)

	session := &fakeSession{}
	claim := &fakeClaim{messages: messages}

	err = handler.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Empty(t, producer.messages)
	assert.Len(t, session.marked, 1)
}

func TestAgentHandlerConsumeClaim_SendsToDLQOnProcessingError(t *testing.T) {
	t.Parallel()

	producer := &fakeProducer{}
	handler := newAgentTestHandler(t, brokenProcessor{}, producer)

	req := UpdateLinkAvro{
		ID:          3,
		URL:         "https://example.com",
		Description: "regular payload",
		TgChatIDs:   []int64{100},
	}

	data, err := handler.avro.Marshal(req)
	require.NoError(t, err)

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{Topic: "link.raw-updates", Value: data}
	close(messages)

	session := &fakeSession{}
	claim := &fakeClaim{messages: messages}

	err = handler.ConsumeClaim(session, claim)
	require.NoError(t, err)
	require.Len(t, producer.messages, 1)
	assert.Equal(t, "dead-letter-topic", producer.messages[0].Topic)
	assert.Len(t, producer.messages[0].Headers, 1)
	assert.Equal(t, []byte("x-dlq-reason"), producer.messages[0].Headers[0].Key)
	assert.Len(t, session.marked, 1)
}
