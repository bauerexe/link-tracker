package botapp

import (
	"errors"

	handlers_pkg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/bot/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

const (
	CommandStart   domain.Command = "start"
	CommandHelp    domain.Command = "help"
	CommandTrack   domain.Command = "track"
	CommandUntrack domain.Command = "untrack"
	CommandList    domain.Command = "list"
)

var ErrorUnknownCommand = errors.New("error unknown command")

func NewBotDispatcher(handlers map[domain.Command]domain.Handler) *BotDispatcher {
	if handlers == nil {
		handlers = map[domain.Command]domain.Handler{
			CommandStart: &handlers_pkg.StartHandler{},
			CommandHelp:  &handlers_pkg.HelpHandler{},
		}
	}
	return &BotDispatcher{handlers: handlers}
}

// BotDispatcher - dispatcher of commands and them handlers
type BotDispatcher struct {
	handlers map[domain.Command]domain.Handler
}

// Dispatch - return of the handler's work
func (r *BotDispatcher) Dispatch(message domain.Message, cmd domain.Command, args string) (string, error) {
	h, ok := r.handlers[cmd]
	if !ok {
		return "", ErrorUnknownCommand
	}
	return h.Handle(message.ChatID, args)
}
