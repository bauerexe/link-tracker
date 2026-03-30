package checkers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"go.uber.org/zap"
)

// StackOverflowChecker - implementation of scrapper_app.Checker for stack overflow links
type StackOverflowChecker struct {
	httpClient *http.Client
	questionRe *regexp.Regexp
	site       string
	key        string
	log        *zap.Logger
}

const (
	stackoverflowHTTPTimeout       = 10 * time.Second
	stackOverflowQuestionMatchSize = 2
)

func NewStackOverflowChecker(httpClient *http.Client, key string, log *zap.Logger) *StackOverflowChecker {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: stackoverflowHTTPTimeout}
	}
	return &StackOverflowChecker{
		httpClient: httpClient,
		key:        key,
		site:       "stackoverflow",
		questionRe: regexp.MustCompile(`^https?://stackoverflow\.com/questions/(\d+)(/|$)`),
		log:        log.Named("so_checker"),
	}
}

func (c *StackOverflowChecker) Match(url string) bool {
	return c.questionRe.MatchString(url)
}

func (c *StackOverflowChecker) Check(ctx context.Context, url string, since time.Time) (string, time.Time, bool, error) {
	c.log.Debug("check start",
		zap.String("url", url),
		zap.Time("since", since),
		zap.Bool("has_key", c.key != ""),
	)
	m := c.questionRe.FindStringSubmatch(url)
	if len(m) < stackOverflowQuestionMatchSize {
		return "", time.Time{}, false, fmt.Errorf("invalid stackoverflow url: %s", url)
	}
	qID := m[1]

	apiURL := fmt.Sprintf("https://api.stackexchange.com/2.3/questions/%s?site=%s&filter=default", qID, c.site)
	if c.key != "" {
		apiURL += "&key=" + c.key
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("do request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Warn("failed to close response body", zap.Error(cerr))
		}
	}()

	c.log.Debug("stackexchange response",
		zap.Int("status", resp.StatusCode),
		zap.String("status_text", resp.Status),
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, false, fmt.Errorf("stackexchange api status: %s", resp.Status)
	}

	var r struct {
		Items []struct {
			Title            string `json:"title"`
			Link             string `json:"link"`
			LastActivityDate int64  `json:"last_activity_date"`
		} `json:"items"`
	}

	decodeErr := json.NewDecoder(resp.Body).Decode(&r)
	if decodeErr != nil {
		return "", time.Time{}, false, fmt.Errorf("decode response: %w", decodeErr)
	}

	if len(r.Items) == 0 {
		return "", time.Time{}, false, fmt.Errorf("question not found: %s", url)
	}

	item := r.Items[0]
	updatedAt := time.Unix(item.LastActivityDate, 0).UTC()

	c.log.Debug("parsed",
		zap.Time("updated_at", updatedAt),
		zap.Time("since", since),
	)

	if !since.IsZero() && (updatedAt.Equal(since) || updatedAt.Before(since)) {
		c.log.Debug("no updates")
		return "", updatedAt, false, nil
	}

	desc := fmt.Sprintf("StackOverflow: активность по вопросу \"%s\" (%s). Время: %s",
		item.Title, item.Link, updatedAt.Format(time.RFC3339),
	)
	c.log.Info("update detected", zap.String("url", url), zap.Time("updated_at", updatedAt))
	return desc, updatedAt, true, nil
}
