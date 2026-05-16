package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
)

var authorLineRegexp = regexp.MustCompile(`(?im)^\s*(?:author|автор)\s*:\s*(.+?)\s*$`)

type Update struct {
	ID          int64
	URL         string
	Description string
	TgChatIDs   []int64
}

type Summarizer interface {
	Summarize(ctx context.Context, text string, limit int) (string, error)
}

type StubSummarizer struct{}

func NewStubSummarizer() *StubSummarizer {
	return &StubSummarizer{}
}

func (s *StubSummarizer) Summarize(_ context.Context, text string, limit int) (string, error) {
	if limit <= 0 {
		return text, nil
	}

	runes := []rune(text)
	if len(runes) <= limit {
		return text, nil
	}

	summary := strings.TrimSpace(string(runes[:limit]))
	if summary == "" {
		return "...", nil
	}

	return summary + "...", nil
}

type Processor struct {
	cfg        config.AgentConfig
	summarizer Summarizer
}

func NewProcessor(cfg config.AgentConfig, summarizer Summarizer) *Processor {
	if summarizer == nil {
		summarizer = NewStubSummarizer()
	}

	return &Processor{
		cfg:        cfg,
		summarizer: summarizer,
	}
}

func (p *Processor) Process(ctx context.Context, update Update) (Update, bool, error) {
	update.URL = strings.TrimSpace(update.URL)
	update.Description = strings.TrimSpace(update.Description)

	if p.shouldDropUpdate(update) {
		return update, false, nil
	}

	description, err := p.summarizeDescription(ctx, update.Description)
	if err != nil {
		return update, false, err
	}

	update.Description = description

	return update, true, nil
}

func (p *Processor) summarizeDescription(ctx context.Context, description string) (string, error) {
	threshold := p.cfg.Summarization.Threshold
	if threshold <= 0 || utf8.RuneCountInString(description) <= threshold {
		return description, nil
	}

	summary, err := p.summarizer.Summarize(ctx, description, threshold)
	summary = strings.TrimSpace(summary)

	if err == nil && summary != "" {
		return summary, nil
	}

	fallbackSummary, err := NewStubSummarizer().Summarize(ctx, description, threshold)
	if err != nil {
		return "", fmt.Errorf("fallback summarize update: %w", err)
	}

	fallbackSummary = strings.TrimSpace(fallbackSummary)
	if fallbackSummary == "" {
		return description, nil
	}

	return fallbackSummary, nil
}

func (p *Processor) shouldDropUpdate(update Update) bool {
	return isServiceUpdate(update) || p.shouldDrop(update.Description)
}

func isServiceUpdate(update Update) bool {
	return strings.EqualFold(strings.TrimSpace(update.URL), "failed-links-report")
}

func (p *Processor) shouldDrop(text string) bool {
	return p.hasStopWord(text) || p.hasExcludedAuthor(text) || p.hasShortText(text)
}

func (p *Processor) hasStopWord(text string) bool {
	if text == "" {
		return false
	}

	lowerText := strings.ToLower(text)

	for _, stopWord := range p.cfg.Filtering.StopWords {
		stopWord = strings.TrimSpace(stopWord)
		if stopWord == "" {
			continue
		}

		if strings.Contains(lowerText, strings.ToLower(stopWord)) {
			return true
		}
	}

	return false
}

func (p *Processor) hasExcludedAuthor(text string) bool {
	if len(p.cfg.Filtering.ExcludedAuthors) == 0 {
		return false
	}

	authors := extractAuthors(text)
	if len(authors) == 0 {
		return false
	}

	for _, author := range authors {
		for _, excludedAuthor := range p.cfg.Filtering.ExcludedAuthors {
			if strings.EqualFold(author, strings.TrimSpace(excludedAuthor)) {
				return true
			}
		}
	}

	return false
}

func (p *Processor) hasShortText(text string) bool {
	minLength := p.cfg.Filtering.MinLength
	if minLength <= 0 {
		return false
	}

	return utf8.RuneCountInString(strings.TrimSpace(text)) < minLength
}

func extractAuthors(text string) []string {
	matches := authorLineRegexp.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	authors := make([]string, 0, len(matches))
	for _, match := range matches {
		const expect = 2
		if len(match) < expect {
			continue
		}

		author := strings.TrimSpace(match[1])
		if author == "" {
			continue
		}

		authors = append(authors, author)
	}

	return authors
}
