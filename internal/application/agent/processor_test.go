package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
)

type brokenSummarizer struct{}

func (brokenSummarizer) Summarize(context.Context, string, int) (string, error) {
	return "", errors.New("boom")
}

func TestProcessorFiltersByStopWord(t *testing.T) {
	t.Parallel()

	processor := NewProcessor(config.AgentConfig{
		Filtering: config.AgentFilteringConfig{
			StopWords: []string{"spam"},
		},
	}, NewStubSummarizer())

	_, keep, err := processor.Process(context.Background(), Update{Description: "This contains SPAM content"})
	require.NoError(t, err)
	assert.False(t, keep)
}

func TestProcessorFiltersByExcludedAuthor(t *testing.T) {
	t.Parallel()

	processor := NewProcessor(config.AgentConfig{
		Filtering: config.AgentFilteringConfig{
			ExcludedAuthors: []string{"bot-user"},
		},
	}, NewStubSummarizer())

	_, keep, err := processor.Process(context.Background(), Update{Description: "Автор: bot-user\nОписание: полезный текст"})
	require.NoError(t, err)
	assert.False(t, keep)
}

func TestProcessorFiltersByMinLength(t *testing.T) {
	t.Parallel()

	processor := NewProcessor(config.AgentConfig{
		Filtering: config.AgentFilteringConfig{
			MinLength: 10,
		},
	}, NewStubSummarizer())

	_, keep, err := processor.Process(context.Background(), Update{Description: "short"})
	require.NoError(t, err)
	assert.False(t, keep)
}

func TestProcessorSummarizesLongDescription(t *testing.T) {
	t.Parallel()

	processor := NewProcessor(config.AgentConfig{
		Summarization: config.AgentSummarizationConfig{
			Threshold: 20,
		},
	}, NewStubSummarizer())

	processed, keep, err := processor.Process(context.Background(), Update{Description: strings.Repeat("a", 30)})
	require.NoError(t, err)
	assert.True(t, keep)
	assert.Equal(t, strings.Repeat("a", 20)+"...", processed.Description)
}

func TestProcessorFallsBackToStubWhenSummarizerFails(t *testing.T) {
	t.Parallel()

	processor := NewProcessor(config.AgentConfig{
		Summarization: config.AgentSummarizationConfig{
			Threshold: 5,
		},
	}, brokenSummarizer{})

	processed, keep, err := processor.Process(context.Background(), Update{Description: "123456"})

	require.NoError(t, err)
	assert.True(t, keep)
	assert.Equal(t, "12345...", processed.Description)
}

func TestExtractAuthorsSupportsEnglishAndRussianLabels(t *testing.T) {
	t.Parallel()

	authors := extractAuthors("Author: alice\nАвтор: bob")
	assert.Equal(t, []string{"alice", "bob"}, authors)
}
