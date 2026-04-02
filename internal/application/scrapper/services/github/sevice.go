package github

import (
	"context"
	"errors"
	"time"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"go.uber.org/zap"
)

type gitHubChecker struct {
	GitHubClient     Client
	GitHubRepository Repository
	Log              *zap.Logger
}

func NewGitHubChecker(githubClient Client, githubRepository Repository, log *zap.Logger) (scrapperapp.Checker, error) {
	if githubClient == nil {
		return nil, errors.New("GitHub client is nil")
	}
	if githubRepository == nil {
		return nil, errors.New("GitHub repository is nil")
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &gitHubChecker{
		GitHubClient:     githubClient,
		GitHubRepository: githubRepository,
		Log:              log,
	}, nil
}

func (g gitHubChecker) Match(url string) bool {
	//TODO implement me
	panic("implement me")
}

func (g gitHubChecker) Check(ctx context.Context, url string, since time.Time) (desc string, updatedAt time.Time, updated bool, err error) {
	//TODO implement me
	panic("implement me")
}
