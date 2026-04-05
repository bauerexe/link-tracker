package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/services/stackoverflow"
	"go.uber.org/zap"
)

// StackOverflowClient - implementation of stackoverflow.Client
type StackOverflowClient struct {
	httpClient *http.Client
	site       string
	key        string
	log        *zap.Logger
}

const (
	stackoverflowHTTPTimeout = 10 * time.Second
)

func NewStackOverflowClient(httpClient *http.Client, key string, log *zap.Logger) stackoverflow.Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: stackoverflowHTTPTimeout}
	}
	if log == nil {
		log = zap.NewNop()
	}

	return &StackOverflowClient{
		httpClient: httpClient,
		key:        key,
		site:       "stackoverflow",
		log:        log,
	}
}

func (c *StackOverflowClient) GetQuestion(ctx context.Context, questionID string) (*stackoverflow.Question, error) {
	apiURL := fmt.Sprintf(
		"https://api.stackexchange.com/2.3/questions/%s?site=%s&filter=default",
		questionID,
		c.site,
	)
	if c.key != "" {
		apiURL += "&key=" + c.key
	}

	resp, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Warn("failed to close response body", zap.Error(cerr))
		}
	}()

	c.logResponse(resp)

	if err = c.checkResponseStatus(resp, apiURL); err != nil {
		return nil, err
	}

	var apiResp stackoverflow.QuestionResponse
	if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode question response: %w", err)
	}

	if len(apiResp.Items) == 0 {
		return nil, stackoverflow.ErrQuestionNotFound
	}

	return &apiResp.Items[0], nil
}

func (c *StackOverflowClient) ListAnswers(ctx context.Context, questionID string) ([]stackoverflow.AnswerOrComment, error) {
	apiURL := fmt.Sprintf(
		"https://api.stackexchange.com/2.3/questions/%s/answers?site=%s&filter=withbody&sort=creation&order=desc",
		questionID,
		c.site,
	)
	if c.key != "" {
		apiURL += "&key=" + c.key
	}

	resp, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Warn("failed to close response body", zap.Error(cerr))
		}
	}()

	c.logResponse(resp)

	if err = c.checkResponseStatus(resp, apiURL); err != nil {
		return nil, err
	}

	var apiResp stackoverflow.AnswerResponse
	if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode answers response: %w", err)
	}

	return apiResp.Items, nil
}

func (c *StackOverflowClient) ListComments(ctx context.Context, questionID string) ([]stackoverflow.AnswerOrComment, error) {
	apiURL := fmt.Sprintf(
		"https://api.stackexchange.com/2.3/questions/%s/comments?site=%s&filter=withbody&sort=creation&order=desc",
		questionID,
		c.site,
	)
	if c.key != "" {
		apiURL += "&key=" + c.key
	}

	resp, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Warn("failed to close response body", zap.Error(cerr))
		}
	}()

	c.logResponse(resp)

	if err = c.checkResponseStatus(resp, apiURL); err != nil {
		return nil, err
	}

	var apiResp stackoverflow.CommentResponse
	if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode comments response: %w", err)
	}

	return apiResp.Items, nil
}

func (c *StackOverflowClient) doRequest(ctx context.Context, apiURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Warn("stackexchange request failed", zap.Error(err))
		return nil, fmt.Errorf("do request: %w", err)
	}

	return resp, nil
}

func (c *StackOverflowClient) logResponse(resp *http.Response) {
	c.log.Debug("stackexchange response",
		zap.Int("status", resp.StatusCode),
		zap.String("status_text", resp.Status),
	)
}

func (c *StackOverflowClient) checkResponseStatus(resp *http.Response, url string) error {
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	var body struct {
		ErrorID      int    `json:"error_id"`
		ErrorName    string `json:"error_name"`
		ErrorMessage string `json:"error_message"`
	}

	_ = json.NewDecoder(resp.Body).Decode(&body)

	c.log.Warn("stackexchange status code <200 or >=300",
		zap.String("url", url),
		zap.Int("status", resp.StatusCode),
		zap.Int("error_id", body.ErrorID),
		zap.String("error_name", body.ErrorName),
		zap.String("error_message", body.ErrorMessage),
	)

	return fmt.Errorf("stackexchange api status: %s", resp.Status)
}
