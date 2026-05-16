package config

import (
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentConfig(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		expected AgentConfig
		init     func(testFs afero.Fs)
		err      error
		negative bool
	}

	initFunc := func(str string) func(testFs afero.Fs) {
		return func(testFs afero.Fs) {
			_ = afero.WriteFile(testFs, "app.env", []byte(str), 0o644)
		}
	}

	testCases := []testCase{
		{
			name: "positive all fields",
			expected: AgentConfig{
				OutputTopic: "link.processed-updates",
				Filtering: AgentFilteringConfig{
					StopWords:       []string{"spam", "ads", "promo"},
					ExcludedAuthors: []string{"bot-user"},
					MinLength:       20,
				},
				Summarization: AgentSummarizationConfig{
					Provider:       "gemini",
					Threshold:      500,
					GeminiAPIKey:   "test-key",
					GeminiModel:    "gemini-2.5-flash-lite",
					RequestTimeout: 12 * time.Second,
				},
			},
			init: initFunc(`
ai_agent {
  output_topic = "link.processed-updates"
  filtering {
    stop_words = ["spam", "ads", "promo"]
    excluded_authors = ["bot-user"]
    min_length = 20
  }
  summarization {
    provider = "gemini"
    threshold = 500
    gemini_api_key = "test-key"
    gemini_model = "gemini-2.5-flash-lite"
    request_timeout = "12s"
  }
}
`),
		},
		{
			name:     "no ai_agent prefix -> empty config",
			init:     initFunc(`output_topic = "link.processed-updates"`),
			expected: AgentConfig{},
		},
		{
			name: "negative parse error",
			init: initFunc(`
ai_agent {
  output_topic = "broken
}
`),
			negative: true,
			err:      ErrParseFile,
		},
		{
			name:     "negative read error file missing",
			init:     func(_ afero.Fs) {},
			negative: true,
			err:      ErrReadFile,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testFs := afero.NewMemMapFs()
			tc.init(testFs)

			cfg, err := NewAgentConfig(testFs)

			if tc.negative {
				require.Error(t, err)
				assert.EqualError(t, err, tc.err.Error())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, cfg)
		})
	}
}
