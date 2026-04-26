package kafka

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type fakeBotGateway struct {
	calls []int64
	errs  map[int64]error
}

func (f *fakeBotGateway) GetMessages(_ context.Context, _ int) (<-chan domain.Message, error) {
	// TODO implement me
	panic("implement me")
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

func (f *fakeSession) Claims() map[string][]int32 {
	return nil
}
func (f *fakeSession) MemberID() string {
	return ""
}
func (f *fakeSession) GenerationID() int32 {
	return 0
}
func (f *fakeSession) MarkOffset(_ string, _ int32, _ int64, _ string)  {}
func (f *fakeSession) ResetOffset(_ string, _ int32, _ int64, _ string) {}
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
func TestHandlerConsumeClaim_Success(t *testing.T) {
	t.Parallel()

	bot := &fakeBotGateway{}
	handler := NewHandler(zap.NewNop(), bot)

	req := &pbv1.UpdateLinkRequest{
		Id:          1,
		Url:         "https://example.com",
		Description: "new update",
		TgChatIds:   []int64{100, 200},
	}

	data, err := proto.Marshal(req)
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
	handler := NewHandler(zap.NewNop(), bot)

	req := &pbv1.UpdateLinkRequest{
		Id:        1,
		Url:       "https://example.com",
		TgChatIds: nil,
	}

	data, err := proto.Marshal(req)
	require.NoError(t, err)

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{Value: data}
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
		req      *pbv1.UpdateLinkRequest
		expected string
	}{
		{
			name: "without description",
			req: &pbv1.UpdateLinkRequest{
				Url: "https://example.com",
			},
			expected: "🔔 Обновление по ссылке:\nhttps://example.com",
		},
		{
			name: "with description",
			req: &pbv1.UpdateLinkRequest{
				Url:         "https://example.com",
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
