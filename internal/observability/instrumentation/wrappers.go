package instrumentation

import (
	"context"
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
	return r.next.CreateChat(ctx, id)
}

func (r *measuredChatRepository) GetChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "chats", started)
	return r.next.GetChatByID(ctx, id)
}

func (r *measuredChatRepository) DeleteChatByID(ctx context.Context, id int64) (*domain.Chat, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "chats", started)
	return r.next.DeleteChatByID(ctx, id)
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
	return r.next.CreateLink(ctx, chatID, rawURL, tags, filters)
}

func (r *measuredLinkRepository) GetLinksByChatID(ctx context.Context, chatID int64, limit, offset uint64) ([]*domain.Link, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)
	return r.next.GetLinksByChatID(ctx, chatID, limit, offset)
}

func (r *measuredLinkRepository) DeleteLink(ctx context.Context, chatID int64, rawURL string) (*domain.Link, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)
	return r.next.DeleteLink(ctx, chatID, rawURL)
}

func (r *measuredLinkRepository) ListLinksBatch(ctx context.Context, limit, offset int) ([]*domain.Link, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)
	return r.next.ListLinksBatch(ctx, limit, offset)
}

func (r *measuredLinkRepository) GetChatIDsByLink(ctx context.Context, rawURL string) ([]int64, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)
	return r.next.GetChatIDsByLink(ctx, rawURL)
}

func (r *measuredLinkRepository) GetURLState(ctx context.Context, rawURL string) (domain.URLState, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)
	return r.next.GetURLState(ctx, rawURL)
}

func (r *measuredLinkRepository) SetURLState(ctx context.Context, rawURL string, st domain.URLState) error {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "links", started)
	return r.next.SetURLState(ctx, rawURL, st)
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
	return r.next.CreateState(ctx, repoFullName, state)
}

func (r *measuredGitHubRepository) GetState(ctx context.Context, repoFullName string) (*github.State, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "github_sync_state", started)
	return r.next.GetState(ctx, repoFullName)
}

func (r *measuredGitHubRepository) UpdateState(ctx context.Context, repoFullName string, state github.State) error {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "github_sync_state", started)
	return r.next.UpdateState(ctx, repoFullName, state)
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
	return c.next.GetRepo(ctx, owner, repo)
}

func (c *measuredGitHubClient) ListIssuesAfter(ctx context.Context, owner, repo string, afterNumber int) ([]*github.Issue, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "github", started)
	return c.next.ListIssuesAfter(ctx, owner, repo, afterNumber)
}

func (c *measuredGitHubClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "github", started)
	return c.next.GetPullRequest(ctx, owner, repo, number)
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
	return c.next.GetQuestion(ctx, questionID)
}

func (c *measuredStackOverflowClient) ListAnswers(ctx context.Context, questionID string) ([]stackoverflow.AnswerOrComment, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "stackoverflow", started)
	return c.next.ListAnswers(ctx, questionID)
}

func (c *measuredStackOverflowClient) ListComments(ctx context.Context, questionID string) ([]stackoverflow.AnswerOrComment, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeExternalSource, "stackoverflow", started)
	return c.next.ListComments(ctx, questionID)
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
	return c.next.CreateChat(ctx, in, opts...)
}

func (c *measuredScrapperClient) DeleteChat(ctx context.Context, in *pbv1.DeleteChatRequest, opts ...grpc.CallOption) (*pbv1.ChatResponse, error) {
	started := time.Now()
	defer observability.ObserveCommandDuration(observability.ScopeScrapperSyncAPI, "DeleteChat", started)
	return c.next.DeleteChat(ctx, in, opts...)
}

func (c *measuredScrapperClient) CreateLink(ctx context.Context, in *pbv1.CreateLinkRequest, opts ...grpc.CallOption) (*pbv1.LinkResponse, error) {
	started := time.Now()
	defer observability.ObserveCommandDuration(observability.ScopeScrapperSyncAPI, "CreateLink", started)
	return c.next.CreateLink(ctx, in, opts...)
}

func (c *measuredScrapperClient) GetLinks(ctx context.Context, in *pbv1.GetLinksRequest, opts ...grpc.CallOption) (*pbv1.ListLinksResponse, error) {
	started := time.Now()
	defer observability.ObserveCommandDuration(observability.ScopeScrapperSyncAPI, "GetLinks", started)
	return c.next.GetLinks(ctx, in, opts...)
}

func (c *measuredScrapperClient) DeleteLink(ctx context.Context, in *pbv1.DeleteLinkRequest, opts ...grpc.CallOption) (*pbv1.LinkResponse, error) {
	started := time.Now()
	defer observability.ObserveCommandDuration(observability.ScopeScrapperSyncAPI, "DeleteLink", started)
	return c.next.DeleteLink(ctx, in, opts...)
}
