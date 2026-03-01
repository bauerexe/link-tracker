package bot_gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type roundTripperRewrite struct {
	base   http.RoundTripper
	target *url.URL
}

func (rt roundTripperRewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	r2 := req.Clone(req.Context())
	r2.URL.Scheme = rt.target.Scheme
	r2.URL.Host = rt.target.Host
	r2.Host = rt.target.Host
	return rt.base.RoundTrip(r2)
}

func TestTelegramRepository_BasicFlow(t *testing.T) {
	t.Parallel()

	const token = "TEST_TOKEN"

	type capturedSend struct {
		chatID string
		text   string
		reply  string
	}

	var (
		mu        sync.Mutex
		gotGetMe  int
		gotSetCmd int
		gotSend   int
		gotUpd    int
		sent      capturedSend
	)

	// Fake Telegram API
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/bot"+token+"/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		method := strings.TrimPrefix(path, "/bot"+token+"/")

		mu.Lock()
		defer mu.Unlock()

		switch method {
		case "getMe":
			gotGetMe++
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Test","username":"test_bot"}}`))
		case "setMyCommands":
			gotSetCmd++
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		case "sendMessage":
			gotSend++
			_ = r.ParseForm()
			sent = capturedSend{
				chatID: r.Form.Get("chat_id"),
				text:   r.Form.Get("text"),
				reply:  r.Form.Get("reply_to_message_id"),
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":8}}`))
		case "getUpdates":
			gotUpd++
			if gotUpd == 1 {
				_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":1000,"message":{"message_id":7,"chat":{"id":42,"type":"private"},"text":"/help"}}]}`))
			} else {
				_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	targetURL := mustParseURL(t, srv.URL)
	old := http.DefaultTransport
	http.DefaultTransport = roundTripperRewrite{base: old, target: targetURL}
	defer func() { http.DefaultTransport = old }()

	repo, err := New(token, zap.NewNop())
	require.NoError(t, err)

	r, ok := repo.(*BotGateway)
	require.True(t, ok)

	mu.Lock()
	assert.GreaterOrEqual(t, gotGetMe, 1)
	assert.GreaterOrEqual(t, gotSetCmd, 1)
	mu.Unlock()

	err = r.SendMessage(42, 7, "reply text")
	require.NoError(t, err)

	mu.Lock()
	assert.Equal(t, 1, gotSend)
	assert.Equal(t, "42", sent.chatID)
	assert.Equal(t, "reply text", sent.text)
	assert.Equal(t, "7", sent.reply)
	mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := r.GetMessages(ctx, 60)
	require.NoError(t, err)

	select {
	case msg := <-ch:
		assert.Equal(t, domain.Message{ChatID: 42, MessageID: 7, Text: "/help"}, msg)
		cancel()
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}
