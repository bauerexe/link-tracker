package config

import (
	"testing"

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
			name: "positive 1",
			expected: ScrapperConfig{
				ScrapperAddrGRPC: "localhost:50051",
				BotAddrGRPC:      "localhost:50052",
			},
			init: initFunc("scrapper {\n scrapper_addr_grpc = \"localhost:50051\"\n bot_addr_grpc = \"localhost:50052\"\n}\n"),
		},
		{
			name: "positive 2 (prod-like)",
			expected: ScrapperConfig{
				ScrapperAddrGRPC: "scrapper:50051",
				BotAddrGRPC:      "bot:50052",
			},
			init: initFunc("scrapper {\n scrapper_addr_grpc = \"scrapper:50051\"\n bot_addr_grpc = \"bot:50052\"\n}\n"),
		},

		{
			name:     "negative (parse error: unclosed quote)",
			init:     initFunc(`scrapper { scrapper_addr_grpc = "localhost:50051 bot_addr_grpc = "localhost:50052" }`),
			negative: true,
			err:      ErrParseFile,
		},

		{
			name:     "no scrapper prefix -> empty cfg, no error",
			init:     initFunc(`scrapper_addr_grpc = "localhost:50051"`),
			expected: ScrapperConfig{},
		},

		{
			name:     "read error: file missing",
			init:     func(testFs afero.Fs) { testFs.Name() },
			negative: true,
			err:      ErrReadFile,
		},
	}

	for _, tc := range testCases {

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
