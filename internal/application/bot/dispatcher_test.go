package botapp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

func TestBotDispatcher_Dispatch(t *testing.T) {
	t.Parallel()
	dp := NewBotDispatcher(map[domain.Command]domain.Handler{
		CommandHelp:  NewHelpHandler(),
		CommandStart: NewStartHandler(),
	})
	type TestCase struct {
		name     string
		command  domain.Command
		positive bool
		err      error
		expected string
	}
	testCases := []TestCase{
		{
			name:     "positive 1",
			command:  CommandHelp,
			positive: true,
			err:      nil,
			expected: "/start — начать\n/help — список команд",
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
