package config

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBotConfig(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		name     string
		expected BotConfig
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
			expected: BotConfig{
				TokenTGBot:       "telegram-token",
				ScrapperAddrGRPC: "localhost:50051",
				BotAddrGRPC:      "localhost:50052",
				BotAddrHTTP:      "localhost:8080",
				TelegramDisabled: true,
			},
			init: initFunc(`
bot {
  app_telegram_token = "telegram-token"
  scrapper_addr_grpc = "localhost:50051"
  bot_addr_grpc = "localhost:50052"
  bot_addr_http = "localhost:8080"
  bot_disable_telegram = true
}
`),
		},
		{
			name: "positive partial fields",
			expected: BotConfig{
				TokenTGBot:       "telegram-token",
				ScrapperAddrGRPC: "scrapper:50051",
			},
			init: initFunc(`
bot {
  app_telegram_token = "telegram-token"
  scrapper_addr_grpc = "scrapper:50051"
}
`),
		},
		{
			name: "no bot prefix -> empty cfg, no error",
			init: initFunc(`
app_telegram_token = "telegram-token"
scrapper_addr_grpc = "localhost:50051"
bot_disable_telegram = true
`),
			expected: BotConfig{},
		},
		{
			name: "negative parse error",
			init: initFunc(`
bot {
  app_telegram_token = "telegram-token
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

			cfg, err := NewBotConfig(testFs)

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
