package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	"go.uber.org/zap"
)

// GitHubClient - implementation of scrapperapp.GitHubClient
type GitHubClient struct {
	httpClient *http.Client
	token      string
	log        *zap.Logger
}

const (
	githubHTTPTimeout = 10 * time.Second
)

func NewGitHubClient(httpClient *http.Client, token string, log *zap.Logger) github.Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: githubHTTPTimeout}
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &GitHubClient{
		httpClient: httpClient,
		token:      token,
		log:        log,
	}
}

func (g *GitHubClient) GetRepo(ctx context.Context, owner, repo string) (*github.Repo, error) {
	var rp github.Repo
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	resp, err := g.doRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			g.log.Warn("failed to close response body", zap.Error(cerr))
		}
	}()

	g.logResponse(resp)

	err = g.checkResponseStatus(resp, apiURL)
	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(resp.Body).Decode(&rp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return &rp, nil
}

func (g *GitHubClient) ListIssuesAfter(ctx context.Context, owner, repo string, afterNumber int) ([]*github.Issue, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?state=all&sort=created&direction=asc", owner, repo)

	resp, err := g.doRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			g.log.Warn("failed to close response body", zap.Error(cerr))
		}
	}()

	g.logResponse(resp)

	if err = g.checkResponseStatus(resp, apiURL); err != nil {
		return nil, err
	}

	var allIssues []*github.Issue
	if err = json.NewDecoder(resp.Body).Decode(&allIssues); err != nil {
		return nil, fmt.Errorf("decode issues response: %w", err)
	}

	result := make([]*github.Issue, 0, len(allIssues))
	for _, issue := range allIssues {
		if issue == nil {
			continue
		}
		if issue.Number > afterNumber {
			result = append(result, issue)
		}
	}

	return result, nil
}

func (g *GitHubClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, number)

	resp, err := g.doRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			g.log.Warn("failed to close response body", zap.Error(cerr))
		}
	}()

	g.logResponse(resp)

	if err = g.checkResponseStatus(resp, apiURL); err != nil {
		return nil, err
	}

	var pr github.PullRequest
	if err = json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode pull request response: %w", err)
	}

	return &pr, nil
}

func (g *GitHubClient) doRequest(ctx context.Context, apiURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)

	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "link-tracker")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.log.Warn("http request failed", zap.Error(err))
		return nil, fmt.Errorf("do request: %w", err)
	}

	return resp, nil
}

func (g *GitHubClient) logResponse(resp *http.Response) {
	g.log.Debug("github response",
		zap.Int("status", resp.StatusCode),
		zap.String("status_text", resp.Status),
		zap.String("x_ratelimit_remaining", resp.Header.Get("X-RateLimit-Remaining")),
		zap.String("x_ratelimit_reset", resp.Header.Get("X-RateLimit-Reset")),
	)
}

func (g *GitHubClient) checkResponseStatus(resp *http.Response, url string) error {
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	var body struct {
		Message          string `json:"message"`
		DocumentationURL string `json:"documentation_url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)

	g.log.Warn("github status code <200 or >=300",
		zap.String("url", url),
		zap.Int("status", resp.StatusCode),
		zap.String("message", body.Message),
		zap.String("doc_url", body.DocumentationURL),
	)

	return fmt.Errorf("github api status: %s", resp.Status)
}
