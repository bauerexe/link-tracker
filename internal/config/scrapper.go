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
	ValkeyAddr           string        `config:"valkey_addr"`
	ValkeyPassword       string        `config:"valkey_password"`
	ValkeyDB             int           `config:"valkey_db"`
	CacheTTL             time.Duration `config:"cache_ttl"`
	CacheEnabled         bool          `config:"cache_enabled"`
	RetryMaxAttempts     uint          `config:"retry_max_attempts"`
	RetryDelay           time.Duration `config:"retry_delay"`
	RetryableStatuses    []int         `config:"retryable_statuses"`
	CBMaxRequests        uint32        `config:"cb_max_requests"`
	CBInterval           time.Duration `config:"cb_interval"`
	CBTimeout            time.Duration `config:"cb_timeout"`
	RateLimitRPS         float64       `config:"rate_limit_rps"`
	RateLimitBurst       int           `config:"rate_limit_burst"`
	HTTPTimeout          time.Duration `config:"http_timeout"`
	CBFailureRate        float64       `config:"cb_failure_rate"`
	CBMinRequests        uint32        `config:"cb_min_requests"`
}

func NewScrapperConfig(fs afero.Fs) (ScrapperConfig, error) {
	file, err := afero.ReadFile(fs, configPath())
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
