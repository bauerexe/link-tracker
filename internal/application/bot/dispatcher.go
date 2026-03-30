package botapp

import (
	"errors"
	"fmt"

	handlers_pkg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

const (
	CommandStart   domain.Command = "/start"
	CommandHelp    domain.Command = "/help"
	CommandTrack   domain.Command = "track"
	CommandUntrack domain.Command = "untrack"
	CommandList    domain.Command = "list"
)

var ErrUnknownCommand = errors.New("error unknown command")

// BotDispatcher - dispatcher of commands and them handlers
type BotDispatcher struct {
	handlers map[domain.Command]domain.Handler
}

func NewBotDispatcher(handlers map[domain.Command]domain.Handler) *BotDispatcher {
	if handlers == nil {
		handlers = map[domain.Command]domain.Handler{
			CommandStart: &handlers_pkg.StartHandler{},
			CommandHelp:  &handlers_pkg.HelpHandler{},
		}
	}
	return &BotDispatcher{handlers: handlers}
}

// Dispatch - return of the handler's work
func (r *BotDispatcher) Dispatch(message domain.Message, cmd domain.Command, args string) (string, error) {
	h, ok := r.handlers[cmd]
	if !ok {
		return "", ErrUnknownCommand
	}
	ans, err := h.Handle(message.ChatID, args)
	if err != nil {
		return "", fmt.Errorf("dispatch err: %w", err)
	}
	return ans, nil
}
