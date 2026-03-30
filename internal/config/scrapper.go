package config

import (
	"github.com/byrnedo/typesafe-config/parse"
	"github.com/spf13/afero"
)

// ScrapperConfig - struct with all that need 'Scrapper Bot Service' to work
type ScrapperConfig struct {
	ScrapperAddrGRPC     string `config:"scrapper_addr_grpc"`
	ScrapperAddrHTTP     string `config:"scrapper_addr_http"`
	BotAddrGRPC          string `config:"bot_addr_grpc"`
	GitHubToken          string `config:"github_token"`
	StackExchangeKey     string `config:"stack_overflow_key"`
	MinutesIntervalCheck int    `config:"minutes_interval_check"`
}

// NewScrapperConfig - init and parse config file 'app.env' in root, with prefix 'bot'
func NewScrapperConfig(fs afero.Fs) (ScrapperConfig, error) {
	file, err := afero.ReadFile(fs, envFile)
	if err != nil {
		return ScrapperConfig{}, ErrReadFile
	}

	tree, err := parse.ParseBytes(file)
	if err != nil {
		return ScrapperConfig{}, ErrParseFile
	}

	cfg := &ScrapperConfig{}
	parse.Populate(cfg, tree.GetConfig(), "scrapper")
	return *cfg, nil
}
