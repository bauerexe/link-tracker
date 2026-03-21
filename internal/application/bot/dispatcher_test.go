package botapp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type stubHandler struct {
	reply string
	err   error
}

func (h stubHandler) Handle(_ int64, _ string) (string, error) {
	return h.reply, h.err
}

func TestBotDispatcher_Dispatch(t *testing.T) {
	t.Parallel()
	dp := NewBotDispatcher(map[domain.Command]domain.Handler{
		CommandHelp:  handlers.NewHelpHandler(),
		CommandStart: stubHandler{reply: "Привет! Я link-tracker бот. Напиши /help"},
	})
	type TestCase struct {
		name     string
		command  domain.Command
		positive bool
		err      error
		expected string
	}
	handler := handlers.HelpHandler{}
	ans, _ := handler.Handle(1, "")
	testCases := []TestCase{
		{
			name:     "positive 1",
			command:  CommandHelp,
			positive: true,
			err:      nil,
			expected: ans,
		},
		{
			name:     "positive 2",
			command:  CommandStart,
			positive: true,
			err:      nil,
			expected: "Привет! Я link-tracker бот. Напиши /help",
		},
		{
			name:     "negative 1",
			command:  domain.Command("/pam_pam"),
			positive: false,
			err:      ErrorUnknownCommand,
			expected: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dispatch, err := dp.Dispatch(domain.Message{ChatID: 1}, tc.command, "")
			if err != nil && tc.positive {
				assert.Error(t, err, "err in positive test")
			} else if err != nil {
				assert.EqualError(t, err, tc.err.Error())
			}
			if !tc.positive {
				assert.Error(t, fmt.Errorf("expected err"))
			} else {
				assert.Equal(t, tc.expected, dispatch)
			}
		})
	}
}
