package scrappercontroller

import (
	"go.uber.org/zap"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

type api struct {
	log            *zap.Logger
	chatRepository usecase.ChatRepository
	linkRepository usecase.LinkRepository
}

// New - return implementation of pbv1.ScrapperServer
func New(log *zap.Logger, chatRepo usecase.ChatRepository, linkRepo usecase.LinkRepository) pbv1.ScrapperServer {
	return &api{
		log:            log,
		chatRepository: chatRepo,
		linkRepository: linkRepo,
	}
}
