package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	"go.uber.org/zap"
)

// GitHubClient - implementation of scrapperapp.GitHubClient
type GitHubClient struct {
	httpClient *http.Client
	token      string
	log        *zap.Logger
	resilience ResilienceConfig
}

const (
	githubHTTPTimeout = 10 * time.Second
)

func NewGitHubClient(httpClient *http.Client, token string, log *zap.Logger, resilience ResilienceConfig) github.Client {
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
		resilience: resilience,
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

func (g *GitHubClient) ListIssuesAfter(
	ctx context.Context,
	owner string,
	repo string,
	afterNumber int,
) ([]*github.Issue, error) {
	const perPage = 100

	result := make([]*github.Issue, 0)

	for page := 1; ; page++ {
		apiURL := fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/issues?state=all&sort=created&direction=asc&per_page=%d&page=%d",
			url.PathEscape(owner),
			url.PathEscape(repo),
			perPage,
			page,
		)

		resp, err := g.doRequest(ctx, apiURL)
		if err != nil {
			return nil, err
		}

		g.logResponse(resp)

		if err = g.checkResponseStatus(resp, apiURL); err != nil {
			if cerr := resp.Body.Close(); cerr != nil {
				g.log.Warn("failed to close response body", zap.Error(cerr))
			}

			return nil, err
		}

		var pageIssues []*github.Issue
		if err = json.NewDecoder(resp.Body).Decode(&pageIssues); err != nil {
			if cerr := resp.Body.Close(); cerr != nil {
				g.log.Warn("failed to close response body", zap.Error(cerr))
			}

			return nil, fmt.Errorf("decode issues response: %w", err)
		}

		if cerr := resp.Body.Close(); cerr != nil {
			g.log.Warn("failed to close response body", zap.Error(cerr))
		}

		g.log.Info("github issues page loaded",
			zap.String("repo", owner+"/"+repo),
			zap.Int("after_number", afterNumber),
			zap.Int("page", page),
			zap.Int("raw_count", len(pageIssues)),
		)

		if len(pageIssues) == 0 {
			break
		}

		for _, issue := range pageIssues {
			if issue == nil {
				continue
			}

			g.log.Info("github issue seen",
				zap.String("repo", owner+"/"+repo),
				zap.Int("issue_number", issue.Number),
				zap.String("title", issue.Title),
			)

			if issue.Number > afterNumber {
				result = append(result, issue)
			}
		}

		if len(pageIssues) < perPage {
			break
		}
	}

	g.log.Info("github issues filtered",
		zap.String("repo", owner+"/"+repo),
		zap.Int("after_number", afterNumber),
		zap.Int("result_count", len(result)),
	)

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

	resp, err := doWithResilience(ctx, g.resilience, func(c context.Context) (*http.Response, error) {
		req = req.WithContext(c)
		return g.httpClient.Do(req)
	})
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
