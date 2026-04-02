package clients

import (
	"context"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/github"
	"go.uber.org/zap"
)

/*
План такой - получаем UpdatedAt от репозитория
затем проверяем что у данного репощитория есть запись в бдшке
если есть - сверяем с сохраненным временем там если там время < чем мое
то идем сверять поменялся ли счетчик issues и pr =>
далее заходим в ишьюсы которые не обработаны проверяем что это пр если пр переходим в пр
апишку и записываем в таблицу изменения(ЕСЛИ НЕ БЫЛО ЗАПИСИ - ОШИБКА, НАДО ПРИ НАЧАЛЕ ОТСЛЕЖИВАНИЯ
РЕПОЗИТОРИЯ СОЗДАВАТЬ ЗАПИСЬ СО ВРЕМЕНЕМ И ПОСЛЕДНИМ СОЗДАННЫМ ИШЬЮСОМ)
*/

// GitHubClient - implementation of scrapperapp.GitHubClient
type GitHubClient struct {
	httpClient *http.Client
	token      string
	//repoRe     *regexp.Regexp
	log *zap.Logger
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

func (g GitHubClient) GetRepo(ctx context.Context, owner, repo string) (github.Repo, error) {
	//TODO implement me
	panic("implement me")
}

func (g GitHubClient) ListIssuesAfter(ctx context.Context, owner, repo string, afterNumber int) ([]github.Issue, error) {
	//TODO implement me
	panic("implement me")
}

func (g GitHubClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (github.PullRequest, error) {
	//TODO implement me
	panic("implement me")
}

//	githubHTTPTimeout   = 10 * time.Second
//	githubRepoMatchSize = 3
//)
//
//func NewGitHubChecker(httpClient *http.Client, token string, log *zap.Logger) *GitHubChecker {
//	if httpClient == nil {
//		httpClient = &http.Client{Timeout: githubHTTPTimeout}
//	}
//	if log == nil {
//		log = zap.NewNop()
//	}
//	return &GitHubChecker{
//		httpClient: httpClient,
//		token:      token,
//		repoRe:     regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/?#]+)`),
//		log:        log.Named("github_checker"),
//	}
//}
//
//func (c *GitHubChecker) Match(url string) bool {
//	return c.repoRe.MatchString(url)
//}
//
//func (c *GitHubChecker) Check(ctx context.Context, url string, since time.Time) (string, time.Time, bool, error) {
//	owner, repo, err := c.parseRepoURL(url)
//	if err != nil {
//		return "", time.Time{}, false, err
//	}
//
//	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
//	c.logCheckStart(url, apiURL, since)
//
//	resp, err := c.doRepoRequest(ctx, apiURL)
//	if err != nil {
//		return "", time.Time{}, false, err
//	}
//	defer func() {
//		if cerr := resp.Body.Close(); cerr != nil {
//			c.log.Warn("failed to close response body", zap.Error(cerr))
//		}
//	}()
//
//	c.logResponse(resp)
//
//	if err = c.checkResponseStatus(resp, url); err != nil {
//		return "", time.Time{}, false, err
//	}
//
//	repoInfo, err := c.decodeRepoResponse(resp)
//	if err != nil {
//		return "", time.Time{}, false, err
//	}
//
//	updatedAt := repoInfo.UpdatedAt
//	c.logParsedTimestamps(repoInfo.UpdatedAt)
//
//	if !since.IsZero() && (updatedAt.Equal(since) || updatedAt.Before(since)) {
//		c.log.Debug("no updates", zap.Time("updated_at", updatedAt), zap.Time("since", since))
//		return "", updatedAt, false, nil
//	}
//
//	desc := fmt.Sprintf(
//		"GitHub: обновление репозитория %s (%s). Время: %s",
//		nonEmpty(repoInfo.FullName, owner+"/"+repo),
//		nonEmpty(repoInfo.HTMLURL, url),
//		updatedAt.Format(time.RFC3339),
//	)
//
//	c.log.Info("update detected",
//		zap.String("repo", nonEmpty(repoInfo.FullName, owner+"/"+repo)),
//		zap.String("url", url),
//		zap.Time("updated_at", updatedAt),
//	)
//
//	return desc, updatedAt, true, nil
//}
//
//func (c *GitHubChecker) parseRepoURL(url string) (string, string, error) {
//	m := c.repoRe.FindStringSubmatch(url)
//	if len(m) != githubRepoMatchSize {
//		return "", "", fmt.Errorf("invalid github repo url: %s", url)
//	}
//
//	return m[1], m[2], nil
//}
//
//func (c *GitHubChecker) logCheckStart(url, apiURL string, since time.Time) {
//	c.log.Debug("check start",
//		zap.String("url", url),
//		zap.String("api_url", apiURL),
//		zap.Time("since", since),
//		zap.Bool("has_token", c.token != ""),
//	)
//}
//
//func (c *GitHubChecker) doRepoRequest(ctx context.Context, apiURL string) (*http.Response, error) {
//	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
//	if err != nil {
//		return nil, fmt.Errorf("create request: %w", err)
//	}
//
//	req.Header.Set("User-Agent", "link-tracker")
//	req.Header.Set("Accept", "application/vnd.github+json")
//	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
//
//	if c.token != "" {
//		req.Header.Set("Authorization", "Bearer "+c.token)
//	}
//
//	resp, err := c.httpClient.Do(req)
//	if err != nil {
//		c.log.Warn("http request failed", zap.Error(err))
//		return nil, fmt.Errorf("do request: %w", err)
//	}
//
//	return resp, nil
//}
//
//func (c *GitHubChecker) logResponse(resp *http.Response) {
//	c.log.Debug("github response",
//		zap.Int("status", resp.StatusCode),
//		zap.String("status_text", resp.Status),
//		zap.String("x_ratelimit_remaining", resp.Header.Get("X-RateLimit-Remaining")),
//		zap.String("x_ratelimit_reset", resp.Header.Get("X-RateLimit-Reset")),
//	)
//}
//
//func (c *GitHubChecker) checkResponseStatus(resp *http.Response, url string) error {
//	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
//		return nil
//	}
//
//	var body struct {
//		Message          string `json:"message"`
//		DocumentationURL string `json:"documentation_url"`
//	}
//	_ = json.NewDecoder(resp.Body).Decode(&body)
//
//	c.log.Warn("github status code <200 or >=300",
//		zap.String("url", url),
//		zap.Int("status", resp.StatusCode),
//		zap.String("message", body.Message),
//		zap.String("doc_url", body.DocumentationURL),
//	)
//
//	return fmt.Errorf("github api status: %s", resp.Status)
//}
//
//type githubRepoResponse struct {
//	FullName        string    `json:"full_name"`
//	HTMLURL         string    `json:"html_url"`
//	UpdatedAt       time.Time `json:"updated_at"`
//	HasIssues       bool      `json:"has_issues"`
//	OpenIssuesCount int       `json:"open_issues_count"`
//	IssuesURL       string    `json:"issues_url"`
//}
//
//type githubIssueResponse struct {
//	PullRequest struct {
//		URL string `json:"url"`
//	} `json:"pull_request"`
//	CreatedAt time.Time `json:"created_at"`
//}
//
//type githubPullRequestResponse struct{}
//
//func (c *GitHubChecker) decodeRepoResponse(resp *http.Response) (githubRepoResponse, error) {
//	var r githubRepoResponse
//
//	err := json.NewDecoder(resp.Body).Decode(&r)
//	if err != nil {
//		c.log.Warn("decode failed", zap.Error(err))
//		return githubRepoResponse{}, fmt.Errorf("decode response: %w", err)
//	}
//
//	return r, nil
//}
//
//func (c *GitHubChecker) logParsedTimestamps(updatedAt time.Time) {
//	c.log.Debug("parsed repo timestamps",
//		zap.Time("updated_at", updatedAt),
//	)
//}
//
//func nonEmpty(v, fallback string) string {
//	v = strings.TrimSpace(v)
//	if v == "" {
//		return fallback
//	}
//	return v
//}
