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

	requestBody := geminiGenerateContentRequest{
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
			Temperature:     0.2,
			MaxOutputTokens: outputTokenLimit(limit),
		},
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal gemini request: %w", err)
	}

	endpoint := fmt.Sprintf(
		"%s/v1beta/models/%s:generateContent?key=%s",
		s.baseURL,
		url.PathEscape(s.model),
		url.QueryEscape(s.apiKey),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("new gemini request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call gemini api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("gemini api status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read gemini response: %w", err)
	}

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
		const defaultLimit = 500
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
		return 256
	}

	tokens := charLimit / 3
	if tokens < 128 {
		return 128
	}

	if tokens > 1024 {
		return 1024
	}

	return tokens
}
