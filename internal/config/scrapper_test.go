package config

import (
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrapperConfig(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		name     string
		expected ScrapperConfig
		init     func(testFs afero.Fs)
		err      error
		negative bool
	}

	initFunc := func(str string) func(testFs afero.Fs) {
		return func(testFs afero.Fs) {
			_ = afero.WriteFile(testFs, "app.env", []byte(str), 0o644)
		}
	}

	testCases := []TestCase{
		{
			name: "positive all fields",
			expected: ScrapperConfig{
				ScrapperAddrGRPC:     "localhost:50051",
				ScrapperAddrHTTP:     "localhost:8080",
				BotAddrGRPC:          "localhost:50052",
				GitHubToken:          "github-token",
				StackExchangeKey:     "stack-key",
				MinutesIntervalCheck: 15,
				PostgresDSN:          "postgres://user:pass@localhost:5432/db?sslmode=disable",
				MigrationsPath:       "file://migrations",
				DBAccessType:         "sql",
				SchedulerInterval:    time.Minute,
				BatchSize:            100,
				WorkerCount:          4,
			},
			init: initFunc(`
scrapper {
  scrapper_addr_grpc = "localhost:50051"
  scrapper_addr_http = "localhost:8080"
  bot_addr_grpc = "localhost:50052"
  github_token = "github-token"
  stack_overflow_key = "stack-key"
  minutes_interval_check = 15
  postgres_dsn = "postgres://user:pass@localhost:5432/db?sslmode=disable"
  migrations_path = "file://migrations"
  db_access_type = "sql"
  scheduler_interval = "1m"
  batch_size = 100
  worker_count = 4
}
`),
		},
		{
			name: "positive partial fields",
			expected: ScrapperConfig{
				ScrapperAddrGRPC: "scrapper:50051",
				BotAddrGRPC:      "bot:50052",
			},
			init: initFunc(`
scrapper {
  scrapper_addr_grpc = "scrapper:50051"
  bot_addr_grpc = "bot:50052"
}
`),
		},
		{
			name: "no scrapper prefix -> empty cfg, no error",
			init: initFunc(`
scrapper_addr_grpc = "localhost:50051"
bot_addr_grpc = "localhost:50052"
`),
			expected: ScrapperConfig{},
		},
		{
			name: "negative parse error",
			init: initFunc(`
scrapper {
  scrapper_addr_grpc = "localhost:50051
}
`),
			negative: true,
			err:      ErrParseFile,
		},
		{
			name: "negative read error file missing",
			init: func(testFs afero.Fs) {
				// intentionally do not create app.env
			},
			negative: true,
			err:      ErrReadFile,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testFs := afero.NewMemMapFs()
			tc.init(testFs)

			cfg, err := NewScrapperConfig(testFs)

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
