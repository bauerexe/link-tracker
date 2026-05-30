package instrumentation

import (
	"context"
	"fmt"
	"time"

	scrapperapp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/stackoverflow"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/observability"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
	"google.golang.org/grpc"
)

type measuredChatRepository struct {
	next scrapperapp.ChatRepository
}

func NewMeasuredChatRepository(next scrapperapp.ChatRepository) scrapperapp.ChatRepository {
	return &measuredChatRepository{next: next}
}

func (r *measuredChatRepository) CreateChat(ctx context.Context, id int64) (*domain.Chat, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "chats", started)

	chat, err := r.next.CreateChat(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("create chat: %w", err)
	}

	return chat, nil
}

func (r *measuredChatRepository) GetChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "chats", started)

	chat, err := r.next.GetChatByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get chat by id: %w", err)
	}

	return chat, nil
}

func (r *measuredChatRepository) DeleteChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "chats", started)

	chat, err := r.next.DeleteChatByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("delete chat by id: %w", err)
	}

	return chat, nil
}

type measuredLinkRepository struct {
	next scrapperapp.LinkRepository
}

func NewMeasuredLinkRepository(next scrapperapp.LinkRepository) scrapperapp.LinkRepository {
	return &measuredLinkRepository{next: next}
}

func (r *measuredLinkRepository) CreateLink(ctx context.Context, chatID int64, rawURL string, tags, filters []string) (*domain.Link, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)

	link, err := r.next.CreateLink(ctx, chatID, rawURL, tags, filters)
	if err != nil {
		return nil, fmt.Errorf("create link: %w", err)
	}

	return link, nil
}

func (r *measuredLinkRepository) GetLinksByChatID(ctx context.Context, chatID int64, limit, offset uint64) ([]*domain.Link, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)

	links, err := r.next.GetLinksByChatID(ctx, chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get links by chat id: %w", err)
	}

	return links, nil
}

func (r *measuredLinkRepository) DeleteLink(ctx context.Context, chatID int64, rawURL string) (*domain.Link, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)

	link, err := r.next.DeleteLink(ctx, chatID, rawURL)
	if err != nil {
		return nil, fmt.Errorf("delete link: %w", err)
	}

	return link, nil
}

func (r *measuredLinkRepository) ListLinksBatch(ctx context.Context, limit, offset int) ([]*domain.Link, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)

	links, err := r.next.ListLinksBatch(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list links batch: %w", err)
	}

	return links, nil
}

func (r *measuredLinkRepository) GetChatIDsByLink(ctx context.Context, rawURL string) ([]int64, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)

	chatIDs, err := r.next.GetChatIDsByLink(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("get chat ids by link: %w", err)
	}

	return chatIDs, nil
}

func (r *measuredLinkRepository) GetURLState(ctx context.Context, rawURL string) (domain.URLState, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)

	state, err := r.next.GetURLState(ctx, rawURL)
	if err != nil {
		return domain.URLState{}, fmt.Errorf("get url state: %w", err)
	}

	return state, nil
}

func (r *measuredLinkRepository) SetURLState(ctx context.Context, rawURL string, st domain.URLState) error {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)

	if err := r.next.SetURLState(ctx, rawURL, st); err != nil {
		return fmt.Errorf("set url state: %w", err)
	}

	return nil
}

type measuredGitHubRepository struct {
	next github.Repository
}

func NewMeasuredGitHubRepository(next github.Repository) github.Repository {
	return &measuredGitHubRepository{next: next}
}

func (r *measuredGitHubRepository) CreateState(ctx context.Context, repoFullName string, state github.State) error {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "github_sync_state", started)

	if err := r.next.CreateState(ctx, repoFullName, state); err != nil {
		return fmt.Errorf("create github state: %w", err)
	}

	return nil
}

func (r *measuredGitHubRepository) GetState(ctx context.Context, repoFullName string) (*github.State, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "github_sync_state", started)

	state, err := r.next.GetState(ctx, repoFullName)
	if err != nil {
		return nil, fmt.Errorf("get github state: %w", err)
	}

	return state, nil
}

func (r *measuredGitHubRepository) UpdateState(ctx context.Context, repoFullName string, state github.State) error {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "github_sync_state", started)

	if err := r.next.UpdateState(ctx, repoFullName, state); err != nil {
		return fmt.Errorf("update github state: %w", err)
	}

	return nil
}

type measuredGitHubClient struct {
	next github.Client
}

func NewMeasuredGitHubClient(next github.Client) github.Client {
	return &measuredGitHubClient{next: next}
}

func (c *measuredGitHubClient) GetRepo(ctx context.Context, owner, repo string) (*github.Repo, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "github", started)

	result, err := c.next.GetRepo(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get github repo: %w", err)
	}

	return result, nil
}

func (c *measuredGitHubClient) ListIssuesAfter(ctx context.Context, owner, repo string, afterNumber int) ([]*github.Issue, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "github", started)

	issues, err := c.next.ListIssuesAfter(ctx, owner, repo, afterNumber)
	if err != nil {
		return nil, fmt.Errorf("list github issues after: %w", err)
	}

	return issues, nil
}

func (c *measuredGitHubClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "github", started)

	pr, err := c.next.GetPullRequest(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("get github pull request: %w", err)
	}

	return pr, nil
}

type measuredStackOverflowClient struct {
	next stackoverflow.Client
}

func NewMeasuredStackOverflowClient(next stackoverflow.Client) stackoverflow.Client {
	return &measuredStackOverflowClient{next: next}
}

func (c *measuredStackOverflowClient) GetQuestion(ctx context.Context, questionID string) (*stackoverflow.Question, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "stackoverflow", started)

	question, err := c.next.GetQuestion(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("get stackoverflow question: %w", err)
	}

	return question, nil
}

func (c *measuredStackOverflowClient) ListAnswers(ctx context.Context, questionID string) ([]stackoverflow.AnswerOrComment, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "stackoverflow", started)

	answers, err := c.next.ListAnswers(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("list stackoverflow answers: %w", err)
	}

	return answers, nil
}

func (c *measuredStackOverflowClient) ListComments(ctx context.Context, questionID string) ([]stackoverflow.AnswerOrComment, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "stackoverflow", started)

	comments, err := c.next.ListComments(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("list stackoverflow comments: %w", err)
	}

	return comments, nil
}

type measuredScrapperClient struct {
	next pbv1.ScrapperClient
}

func NewMeasuredScrapperClient(next pbv1.ScrapperClient) pbv1.ScrapperClient {
	return &measuredScrapperClient{next: next}
}

func (c *measuredScrapperClient) CreateChat(ctx context.Context, in *pbv1.CreateChatRequest, opts ...grpc.CallOption) (*pbv1.ChatResponse, error) {
	started := time.Now()
	defer observability.ObserveCommandDuration(observability.ScopeScrapperSyncAPI, "CreateChat", started)

	resp, err := c.next.CreateChat(ctx, in, opts...)
	if err != nil {
		return nil, fmt.Errorf("create chat in scrapper: %w", err)
	}

	return resp, nil
}

func (c *measuredScrapperClient) DeleteChat(ctx context.Context, in *pbv1.DeleteChatRequest, opts ...grpc.CallOption) (*pbv1.ChatResponse, error) {
	started := time.Now()
	defer observability.ObserveCommandDuration(observability.ScopeScrapperSyncAPI, "DeleteChat", started)

	resp, err := c.next.DeleteChat(ctx, in, opts...)
	if err != nil {
		return nil, fmt.Errorf("delete chat in scrapper: %w", err)
	}

	return resp, nil
}

func (c *measuredScrapperClient) CreateLink(ctx context.Context, in *pbv1.CreateLinkRequest, opts ...grpc.CallOption) (*pbv1.LinkResponse, error) {
	started := time.Now()
	defer observability.ObserveCommandDuration(observability.ScopeScrapperSyncAPI, "CreateLink", started)

	resp, err := c.next.CreateLink(ctx, in, opts...)
	if err != nil {
		return nil, fmt.Errorf("create link in scrapper: %w", err)
	}

	return resp, nil
}

func (c *measuredScrapperClient) GetLinks(ctx context.Context, in *pbv1.GetLinksRequest, opts ...grpc.CallOption) (*pbv1.ListLinksResponse, error) {
	started := time.Now()
	defer observability.ObserveCommandDuration(observability.ScopeScrapperSyncAPI, "GetLinks", started)

	resp, err := c.next.GetLinks(ctx, in, opts...)
	if err != nil {
		return nil, fmt.Errorf("get links from scrapper: %w", err)
	}

	return resp, nil
}

func (c *measuredScrapperClient) DeleteLink(ctx context.Context, in *pbv1.DeleteLinkRequest, opts ...grpc.CallOption) (*pbv1.LinkResponse, error) {
	started := time.Now()
	defer observability.ObserveCommandDuration(observability.ScopeScrapperSyncAPI, "DeleteLink", started)

	resp, err := c.next.DeleteLink(ctx, in, opts...)
	if err != nil {
		return nil, fmt.Errorf("delete link in scrapper: %w", err)
	}

	return resp, nil
}
