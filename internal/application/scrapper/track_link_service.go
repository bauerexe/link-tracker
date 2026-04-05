package scrapperapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"go.uber.org/zap"
)

type GitHubStateRepository interface {
	CreateState(ctx context.Context, repoFullName string, state github.State) error
	GetState(ctx context.Context, repoFullName string) (*github.State, error)
	UpdateState(ctx context.Context, repoFullName string, state github.State) error
}

type TrackLinkService struct {
	chatRepository   ChatRepository
	linkRepository   LinkRepository
	githubRepository GitHubStateRepository
	githubClient     github.Client
	log              *zap.Logger
}

const (
	matchParts = 2
)

func NewTrackLinkService(
	chatRepository ChatRepository,
	linkRepository LinkRepository,
	githubRepository GitHubStateRepository,
	githubClient github.Client,
	log *zap.Logger,
) *TrackLinkService {
	return &TrackLinkService{
		chatRepository:   chatRepository,
		linkRepository:   linkRepository,
		githubRepository: githubRepository,
		githubClient:     githubClient,
		log:              log,
	}
}

func (s *TrackLinkService) CreateLink(ctx context.Context, chatID int64, rawURL string, tags []string, filters []string,
) (*domain.Link, error) {
	if _, err := s.chatRepository.GetChatByID(ctx, chatID); err != nil {
		return nil, fmt.Errorf("failed to get chat id: %w", err)
	}

	createdLink, err := s.linkRepository.CreateLink(ctx, chatID, rawURL, tags, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to create link: %w", err)
	}

	now := time.Now().UTC()

	err = s.linkRepository.SetURLState(ctx, createdLink.URL, domain.URLState{
		LastCheckedAt: now,
		LastUpdatedAt: now,
	})
	s.log.Info("time of url state",
		zap.String("url", createdLink.URL),
		zap.String("lastCheckedAt", now.Format(time.RFC3339)),
		zap.String("lastUpdatedAt", now.Format(time.RFC3339)))

	if err != nil && !errors.Is(err, ErrLinkNotFound) {
		return nil, fmt.Errorf("set url state: %w", err)
	}

	owner, repo, ok := parseGitHubRepo(createdLink.URL)
	if !ok {
		return createdLink, nil
	}

	state := github.State{
		LastRepoUpdated: &now,
	}

	s.log.Info("time of github repo state",
		zap.String("LastRepoUpdated", now.Format(time.RFC3339)),
	)

	err = s.githubRepository.CreateState(ctx, owner+"/"+repo, state)
	if err != nil && !errors.Is(err, github.ErrStateAlreadyExists) {
		return nil, fmt.Errorf("create github state: %w", err)
	}

	return createdLink, nil
}

func parseGitHubRepo(raw string) (owner string, repo string, ok bool) {
	const prefix = "https://github.com/"

	switch {
	case strings.HasPrefix(raw, prefix):
		raw = strings.TrimPrefix(raw, prefix)
	default:
		return "", "", false
	}

	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) < matchParts {
		return "", "", false
	}

	return parts[0], parts[1], true
}
