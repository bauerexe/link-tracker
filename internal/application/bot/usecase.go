package botapp

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type BotRepository interface {
	GetMessages(ctx context.Context, timeoutSec int) (<-chan domain.Message, error)
	SendMessage(chatID int64, replyToMessageID int, text string) error
}
