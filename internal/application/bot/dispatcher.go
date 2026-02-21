package botapp

import (
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

const (
	CommandStart domain.Command = "/start"
	CommandHelp  domain.Command = "/help"
)

// BotDispatcher - dispatcher of commands and them handlers
type BotDispatcher struct {
	handlers map[domain.Command]domain.Handler
}

func NewBotDispatcher(handlers map[domain.Command]domain.Handler) *BotDispatcher {
	if handlers == nil {
		handlers = map[domain.Command]domain.Handler{}
	}
	return &BotDispatcher{handlers: handlers}
}

// StartHandler - handler of message with command - CommandStart
type StartHandler struct{}

func (h *StartHandler) Handle(_ int64, _ string) (string, error) {
	return "Привет! Я link-tracker бот. Напиши /help", nil
}

func NewStartHandler() domain.Handler {
	return &StartHandler{}
}

// HelpHandler - handler of message with command - CommandHelp
type HelpHandler struct{}

func (h *HelpHandler) Handle(_ int64, _ string) (string, error) {
	return "/start — начать\n/help — список команд", nil
}

func NewHelpHandler() domain.Handler {
	return &HelpHandler{}
}

// Dispatch - return of the handler's work
func (r *BotDispatcher) Dispatch(chatID int64, cmd domain.Command, args string) (string, error) {
	h, ok := r.handlers[cmd]
	if !ok {
		return fmt.Sprintf("Не знаю команду %s. Напиши /help", cmd), nil
	}
	return h.Handle(chatID, args)
}
