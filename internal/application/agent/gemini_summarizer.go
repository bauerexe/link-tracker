package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultGeminiModel   = "gemini-2.5-flash-lite"
	defaultGeminiTimeout = 12 * time.Second
	geminiAPIBaseURL     = "https://generativelanguage.googleapis.com"
	defaultLimit         = 500
	temperature          = 0.2
	buf                  = 4096
	readerLimit          = 1 << 20

	defaultGeminiOutputTokens = 256
	geminiCharsPerToken       = 3
	minGeminiOutputTokens     = 128
	maxGeminiOutputTokens     = 1024
)

type GeminiSummarizer struct {
	apiKey  string
	model   string
	timeout time.Duration
	client  *http.Client
	baseURL string
}

type geminiGenerateContentRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason,omitempty"`
	} `json:"candidates"`

	PromptFeedback *struct {
		BlockReason string `json:"blockReason,omitempty"`
	} `json:"promptFeedback,omitempty"`

	Error *struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
		Status  string `json:"status,omitempty"`
	} `json:"error,omitempty"`
}

func NewGeminiSummarizer(apiKey, model string, timeout time.Duration) (*GeminiSummarizer, error) {
	return newGeminiSummarizer(apiKey, model, timeout, nil, geminiAPIBaseURL)
}

func newGeminiSummarizer(
	apiKey string,
	model string,
	timeout time.Duration,
	client *http.Client,
	baseURL string,
) (*GeminiSummarizer, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("gemini api key is empty")
	}

	model = strings.TrimSpace(strings.TrimPrefix(model, "models/"))
	if model == "" {
		model = defaultGeminiModel
	}

	if timeout <= 0 {
		timeout = defaultGeminiTimeout
	}

	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = geminiAPIBaseURL
	}

	return &GeminiSummarizer{
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
		client:  client,
		baseURL: baseURL,
	}, nil
}

func (s *GeminiSummarizer) Summarize(ctx context.Context, text string, limit int) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	body, err := s.generateContent(ctx, text, limit)
	if err != nil {
		return "", err
	}

	summary, err := parseGeminiSummary(body)
	if err != nil {
		return "", err
	}

	return limitGeminiSummary(ctx, summary, limit)
}

func (s *GeminiSummarizer) generateContent(ctx context.Context, text string, limit int) ([]byte, error) {
	req, err := s.newGenerateContentHTTPRequest(ctx, text, limit)
	if err != nil {
		return nil, err
	}

	return s.doGenerateContentRequest(req)
}

func (s *GeminiSummarizer) newGenerateContentHTTPRequest(
	ctx context.Context,
	text string,
	limit int,
) (*http.Request, error) {
	payload, err := json.Marshal(newGeminiGenerateContentRequest(text, limit))
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.generateContentEndpoint(),
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("new gemini request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func newGeminiGenerateContentRequest(text string, limit int) geminiGenerateContentRequest {
	return geminiGenerateContentRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{
						Text: buildGeminiSummarizationPrompt(text, limit),
					},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     temperature,
			MaxOutputTokens: outputTokenLimit(limit),
		},
	}
}

func (s *GeminiSummarizer) generateContentEndpoint() string {
	return fmt.Sprintf(
		"%s/v1beta/models/%s:generateContent?key=%s",
		s.baseURL,
		url.PathEscape(s.model),
		url.QueryEscape(s.apiKey),
	)
}

func (s *GeminiSummarizer) doGenerateContentRequest(req *http.Request) ([]byte, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}

		return nil, fmt.Errorf("call gemini api: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return readGeminiResponseBody(resp)
}

func readGeminiResponseBody(resp *http.Response) ([]byte, error) {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, err := io.ReadAll(io.LimitReader(resp.Body, buf))
		if err != nil {
			return nil, fmt.Errorf("read gemini error response: %w", err)
		}

		return nil, fmt.Errorf("gemini api status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, readerLimit))
	if err != nil {
		return nil, fmt.Errorf("read gemini response: %w", err)
	}

	return body, nil
}

func parseGeminiSummary(body []byte) (string, error) {
	var response geminiGenerateContentResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode gemini response: %w; body=%s", err, strings.TrimSpace(string(body)))
	}

	if response.Error != nil {
		return "", fmt.Errorf(
			"gemini api error: code=%d status=%s message=%s",
			response.Error.Code,
			response.Error.Status,
			response.Error.Message,
		)
	}

	summary := strings.TrimSpace(response.text())
	if summary == "" {
		return "", fmt.Errorf(
			"gemini returned empty summary: finish_reason=%q prompt_block=%q body=%s",
			response.finishReason(),
			response.blockReason(),
			strings.TrimSpace(string(body)),
		)
	}

	return summary, nil
}

func limitGeminiSummary(ctx context.Context, summary string, limit int) (string, error) {
	if limit > 0 && utf8.RuneCountInString(summary) > limit {
		return NewStubSummarizer().Summarize(ctx, summary, limit)
	}

	return summary, nil
}

func (r geminiGenerateContentResponse) text() string {
	for _, candidate := range r.Candidates {
		for _, part := range candidate.Content.Parts {
			if text := strings.TrimSpace(part.Text); text != "" {
				return text
			}
		}
	}

	return ""
}

func (r geminiGenerateContentResponse) finishReason() string {
	for _, candidate := range r.Candidates {
		if candidate.FinishReason != "" {
			return candidate.FinishReason
		}
	}

	return ""
}

func (r geminiGenerateContentResponse) blockReason() string {
	if r.PromptFeedback == nil {
		return ""
	}

	return r.PromptFeedback.BlockReason
}

func buildGeminiSummarizationPrompt(text string, limit int) string {
	if limit <= 0 {

		limit = defaultLimit
	}

	return fmt.Sprintf(`Суммаризируй обновление для Telegram на русском языке в 2-3 коротких предложения.

Требования:
- сохрани суть обновления;
- сохрани важные имена, номера issue/PR, ссылки и технические детали;
- не добавляй вступление вроде "Вот краткое резюме";
- желательная длина: до %d символов.
Формат:
🔔 Обновление по ссылке/ссылкам:
{...}

Обновления в {...}:

Новые {...} (1):
#{nums} issue/pr
Автор: {...}
Создано: 2026-05-16T12:50:02Z
Описание:
{...}

{...} - замени текстом
Текст обновления:
%s`, limit, text)
}

func outputTokenLimit(charLimit int) int {
	if charLimit <= 0 {
		return defaultGeminiOutputTokens
	}

	tokens := charLimit / geminiCharsPerToken
	if tokens < minGeminiOutputTokens {
		return minGeminiOutputTokens
	}

	if tokens > maxGeminiOutputTokens {
		return maxGeminiOutputTokens
	}

	return tokens
}
