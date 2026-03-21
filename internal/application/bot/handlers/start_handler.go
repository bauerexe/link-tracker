package handlers

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

// HelpHandler - handler of message with command - CommandHelp
type HelpHandler struct{}

func (h *HelpHandler) Handle(_ int64, _ string) (string, error) {
	return "/start — начать\n" +
		"/help — список команд\n" +
		"/track — начать отслеживание ссылки. " +
		"Опционально пользователь может указать один или несколько тегов," +
		"привязанных к ссылке.\n" +
		"/untrack — прекратить отслеживание ссылки.\n" +
		"/list — вывести список всех ссылок," +
		"отслеживаемых пользователем." +
		"Опционально вторым параметром можно указать тег" +
		" — в этом случае список ссылок должен быть отфильтрован по указанному тегу.", nil
}

func NewHelpHandler() domain.Handler {
	return &HelpHandler{}
}
