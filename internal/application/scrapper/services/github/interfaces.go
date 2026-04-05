package github

import "context"

type Client interface {
	GetRepo(ctx context.Context, owner, repo string) (*Repo, error)
	ListIssuesAfter(ctx context.Context, owner, repo string, afterNumber int) ([]*Issue, error)
	GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error)
}

type Repository interface {
	CreateState(ctx context.Context, repoFullName string, state State) error
	GetState(ctx context.Context, repoFullName string) (*State, error)
	UpdateState(ctx context.Context, repoFullName string, state State) error
}
