package kafka

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"go.uber.org/zap"
)

const testUpdateLinkSchema = `
{
  "type": "record",
  "name": "UpdateLinkRequest",
  "namespace": "linktracker.kafka",
  "fields": [
    {
      "name": "id",
      "type": "long",
      "default": 0
    },
    {
      "name": "url",
      "type": "string"
    },
    {
      "name": "description",
      "type": "string",
      "default": ""
    },
    {
      "name": "tgChatIds",
      "type": {
        "type": "array",
        "items": "long"
      },
      "default": []
    }
  ]
}
`

type fakeBotGateway struct {
	calls []int64
	errs  map[int64]error
}

func (f *fakeBotGateway) GetMessages(_ context.Context, _ int) (<-chan domain.Message, error) {
	ch := make(chan domain.Message)
	close(ch)

	return ch, nil
}

func (f *fakeBotGateway) SendMessage(chatID int64, _ int, _ string) error {
	f.calls = append(f.calls, chatID)

	if f.errs != nil {
		return f.errs[chatID]
	}

	return nil
}

type fakeSession struct {
	marked []*sarama.ConsumerMessage
}

func (f *fakeSession) Claims() map[string][]int32 { return nil }
func (f *fakeSession) MemberID() string           { return "" }
func (f *fakeSession) GenerationID() int32        { return 0 }
func (f *fakeSession) MarkOffset(_ string, _ int32, _ int64, _ string) {
}
func (f *fakeSession) ResetOffset(_ string, _ int32, _ int64, _ string) {
}
func (f *fakeSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	f.marked = append(f.marked, msg)
}
func (f *fakeSession) Context() context.Context { return context.Background() }
func (f *fakeSession) Commit()                  {}

type fakeClaim struct {
	messages chan *sarama.ConsumerMessage
}

func (f *fakeClaim) Topic() string              { return "updates" }
func (f *fakeClaim) Partition() int32           { return 0 }
func (f *fakeClaim) InitialOffset() int64       { return 0 }
func (f *fakeClaim) HighWaterMarkOffset() int64 { return 0 }
func (f *fakeClaim) Messages() <-chan *sarama.ConsumerMessage {
	return f.messages
}

func newTestHandler(t *testing.T, bot *fakeBotGateway) *Handler {
	t.Helper()

	codec, err := NewAvroCodec(testUpdateLinkSchema)
	require.NoError(t, err)

	return NewHandler(zap.NewNop(), bot, nil, "", 1, codec)
}

func TestHandlerConsumeClaim_Success(t *testing.T) {
	t.Parallel()

	bot := &fakeBotGateway{}
	handler := newTestHandler(t, bot)

	req := UpdateLinkAvro{
		ID:          1,
		URL:         "https://example.com",
		Description: "new update",
		TgChatIDs:   []int64{100, 200},
	}

	data, err := handler.avro.Marshal(req)
	require.NoError(t, err)

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{
		Topic:     "updates",
		Partition: 0,
		Offset:    10,
		Value:     data,
	}
	close(messages)

	session := &fakeSession{}
	claim := &fakeClaim{messages: messages}

	err = handler.ConsumeClaim(session, claim)

	require.NoError(t, err)
	assert.Equal(t, []int64{100, 200}, bot.calls)
	assert.Len(t, session.marked, 1)
}

func TestHandlerConsumeClaim_EmptyChatIDs(t *testing.T) {
	t.Parallel()

	bot := &fakeBotGateway{}
	handler := newTestHandler(t, bot)

	req := UpdateLinkAvro{
		ID:        1,
		URL:       "https://example.com",
		TgChatIDs: nil,
	}

	data, err := handler.avro.Marshal(req)
	require.NoError(t, err)

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{
		Topic:     "updates",
		Partition: 0,
		Offset:    11,
		Value:     data,
	}
	close(messages)

	session := &fakeSession{}
	claim := &fakeClaim{messages: messages}

	err = handler.ConsumeClaim(session, claim)

	require.NoError(t, err)
	assert.Empty(t, bot.calls)
	assert.Len(t, session.marked, 1)
}

func TestFormatUpdate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		req      *UpdateLinkAvro
		expected string
	}{
		{
			name: "without description",
			req: &UpdateLinkAvro{
				URL: "https://example.com",
			},
			expected: "🔔 Обновление по ссылке:\nhttps://example.com",
		},
		{
			name: "with description",
			req: &UpdateLinkAvro{
				URL:         "https://example.com",
				Description: "new answer",
			},
			expected: "🔔 Обновление по ссылке:\nhttps://example.com\n\nnew answer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, formatUpdate(tc.req))
		})
	}
}
