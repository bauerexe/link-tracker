package scrapperapp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"go.uber.org/zap"
)

type TrackLinkService struct {
	chatRepository   ChatRepository
	linkRepository   LinkRepository
	githubRepository github.Repository
	githubClient     github.Client
	log              *zap.Logger
}

func NewTrackLinkService(
	chatRepository ChatRepository,
	linkRepository LinkRepository,
	githubRepository github.Repository,
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

func (s *TrackLinkService) CreateLink(
	ctx context.Context,
	chatID int64,
	rawURL string,
	tags []string,
	filters []string,
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
	if err != nil && !errors.Is(err, ErrLinkNotFound) {
		return nil, fmt.Errorf("set url state: %w", err)
	}

	s.log.Info("time of url state",
		zap.String("url", createdLink.URL),
		zap.String("lastCheckedAt", now.Format(time.RFC3339)),
		zap.String("lastUpdatedAt", now.Format(time.RFC3339)),
	)

	owner, repo, ok := parseGitHubRepo(createdLink.URL)
	if !ok {
		return createdLink, nil
	}

	state, err := s.getInitialGitHubState(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get initial github state: %w", err)
	}

	if state.LastRepoUpdated != nil {
		s.log.Info("time of github repo state",
			zap.String("LastRepoUpdated", state.LastRepoUpdated.Format(time.RFC3339)),
			zap.String("repo", owner+"/"+repo),
		)
	}

	err = s.githubRepository.CreateState(ctx, owner+"/"+repo, state)
	if err != nil && !errors.Is(err, github.ErrStateAlreadyExists) {
		return nil, fmt.Errorf("create github state: %w", err)
	}

	return createdLink, nil
}
func parseGitHubRepo(raw string) (owner string, repo string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		return "", "", false
	}

	if !strings.EqualFold(u.Host, "github.com") {
		return "", "", false
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

func (s *TrackLinkService) getInitialGitHubState(
	ctx context.Context,
	owner string,
	repo string,
) (github.State, error) {
	repoInfo, err := s.githubClient.GetRepo(ctx, owner, repo)
	if err != nil {
		return github.State{}, fmt.Errorf("get github repo: %w", err)
	}

	lastRepoUpdated := repoInfo.UpdatedAt.UTC()

	issues, err := s.githubClient.ListIssuesAfter(ctx, owner, repo, 0)
	if err != nil {
		return github.State{}, fmt.Errorf("list initial github issues: %w", err)
	}

	lastIssueNumber := 0
	for _, issue := range issues {
		if issue.Number > lastIssueNumber {
			lastIssueNumber = issue.Number
		}
	}

	return github.State{
		LastRepoUpdated:          &lastRepoUpdated,
		LastProcessedIssueNumber: lastIssueNumber,
	}, nil
}
