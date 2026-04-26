package config

import (
	"time"

	"github.com/byrnedo/typesafe-config/parse"
	"github.com/spf13/afero"
)

type ScrapperConfig struct {
	ScrapperAddrGRPC     string        `config:"scrapper_addr_grpc"`
	ScrapperAddrHTTP     string        `config:"scrapper_addr_http"`
	BotAddrGRPC          string        `config:"bot_addr_grpc"`
	GitHubToken          string        `config:"github_token"`
	StackExchangeKey     string        `config:"stack_overflow_key"`
	SecondsIntervalCheck int           `config:"seconds_interval_check"`
	PostgresDSN          string        `config:"postgres_dsn"`
	MigrationsPath       string        `config:"migrations_path"`
	DBAccessType         string        `config:"db_access_type"`
	SchedulerInterval    time.Duration `config:"scheduler_interval"`
	BatchSize            int           `config:"batch_size"`
	WorkerCount          int           `config:"worker_count"`
}

func NewScrapperConfig(fs afero.Fs) (ScrapperConfig, error) {
	file, err := afero.ReadFile(fs, EnvFile)
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
