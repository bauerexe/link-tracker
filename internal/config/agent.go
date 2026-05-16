package config

import (
	"time"

	"github.com/byrnedo/typesafe-config/parse"
	"github.com/spf13/afero"
)

const (
	AgentSummarizationProviderStub   = "stub"
	AgentSummarizationProviderGemini = "gemini"
)

type AgentConfig struct {
	OutputTopic   string                   `config:"output_topic"`
	Filtering     AgentFilteringConfig     `config:"filtering"`
	Summarization AgentSummarizationConfig `config:"summarization"`
}

type AgentFilteringConfig struct {
	StopWords       []string `config:"stop_words"`
	ExcludedAuthors []string `config:"excluded_authors"`
	MinLength       int      `config:"min_length"`
}

type AgentSummarizationConfig struct {
	Provider       string        `config:"provider"`
	Threshold      int           `config:"threshold"`
	GeminiAPIKey   string        `config:"gemini_api_key"`
	GeminiModel    string        `config:"gemini_model"`
	RequestTimeout time.Duration `config:"request_timeout"`
}

func NewAgentConfig(fs afero.Fs) (AgentConfig, error) {
	file, err := afero.ReadFile(fs, configPath())
	if err != nil {
		return AgentConfig{}, ErrReadFile
	}

	tree, err := parse.ParseBytes(file)
	if err != nil {
		return AgentConfig{}, ErrParseFile
	}

	cfg := &AgentConfig{}
	parse.Populate(cfg, tree.GetConfig(), "ai_agent")

	return *cfg, nil
}
