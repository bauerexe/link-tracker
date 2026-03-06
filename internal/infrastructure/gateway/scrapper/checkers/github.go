package checkers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
)

// GitHubChecker - implementation of scrapper_app.Checker for github links
type GitHubChecker struct {
	httpClient *http.Client
	token      string
	repoRe     *regexp.Regexp
	log        *zap.Logger
}

func NewGitHubChecker(httpClient *http.Client, token string, log *zap.Logger) *GitHubChecker {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &GitHubChecker{
		httpClient: httpClient,
		token:      token,
		repoRe:     regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/?#]+)`),
		log:        log.Named("github_checker"),
	}
}

func (c *GitHubChecker) Match(url string) bool {
	return c.repoRe.MatchString(url)
}

func (c *GitHubChecker) Check(ctx context.Context, url string, since time.Time) (string, time.Time, bool, error) {
	m := c.repoRe.FindStringSubmatch(url)
	if len(m) != 3 {
		return "", time.Time{}, false, fmt.Errorf("invalid github repo url: %s", url)
	}

	owner := m[1]
	repo := m[2]
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	c.log.Debug("check start",
		zap.String("url", url),
		zap.String("api_url", apiURL),
		zap.Time("since", since),
		zap.Bool("has_token", c.token != ""),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", time.Time{}, false, err
	}

	req.Header.Set("User-Agent", "link-tracker")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Warn("http request failed", zap.Error(err))
		return "", time.Time{}, false, err
	}
	defer resp.Body.Close()

	c.log.Debug("github response",
		zap.Int("status", resp.StatusCode),
		zap.String("status_text", resp.Status),
		zap.String("x_ratelimit_remaining", resp.Header.Get("X-RateLimit-Remaining")),
		zap.String("x_ratelimit_reset", resp.Header.Get("X-RateLimit-Reset")),
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body struct {
			Message          string `json:"message"`
			DocumentationURL string `json:"documentation_url"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)

		c.log.Warn("github status code <200 or >=300",
			zap.String("url", url),
			zap.Int("status", resp.StatusCode),
			zap.String("message", body.Message),
			zap.String("doc_url", body.DocumentationURL),
		)

		return "", time.Time{}, false, fmt.Errorf("github api status: %s", resp.Status)
	}

	var r struct {
		FullName  string    `json:"full_name"`
		HTMLURL   string    `json:"html_url"`
		PushedAt  time.Time `json:"pushed_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		c.log.Warn("decode failed", zap.Error(err))
		return "", time.Time{}, false, err
	}

	updatedAt := r.PushedAt
	if updatedAt.IsZero() {
		updatedAt = r.UpdatedAt
	}

	c.log.Debug("parsed repo timestamps",
		zap.Time("pushed_at", r.PushedAt),
		zap.Time("updated_at", r.UpdatedAt),
		zap.Time("chosen_updated_at", updatedAt),
	)

	if !since.IsZero() && (updatedAt.Equal(since) || updatedAt.Before(since)) {
		c.log.Debug("no updates", zap.Time("updated_at", updatedAt), zap.Time("since", since))
		return "", updatedAt, false, nil
	}

	desc := fmt.Sprintf(
		"GitHub: обновление репозитория %s (%s). Время: %s",
		nonEmpty(r.FullName, owner+"/"+repo),
		nonEmpty(r.HTMLURL, url),
		updatedAt.Format(time.RFC3339),
	)

	c.log.Info("update detected",
		zap.String("repo", nonEmpty(r.FullName, owner+"/"+repo)),
		zap.String("url", url),
		zap.Time("updated_at", updatedAt),
	)

	return desc, updatedAt, true, nil
}

func nonEmpty(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}
