package config

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		name     string
		expected string
		init     func(testFs afero.Fs)
		err      error
		positive bool
	}
	initFunc := func(str string) func(testFs afero.Fs) {
		return func(testFs afero.Fs) {
			_ = afero.WriteFile(testFs, "app.env", []byte(str), 0o644)
		}
	}

	testCases := []TestCase{{
		name:     "positive 1",
		expected: "TEST_TOKEN",
		init:     initFunc(`bot {app_telegram_token = "TEST_TOKEN"}`),
		positive: true,
	}, {
		name:     "positive 2",
		expected: "TEST::TOKEN",
		init:     initFunc(`bot {app_telegram_token = "TEST::TOKEN"}`),
		positive: true,
	}, {
		name:     "negative 1",
		expected: "",
		init:     initFunc(`bot {app_telegram_token = "TEST_TOKEN}`),
		positive: false,
		err:      ErrParseFile,
	}, {
		name:     "negative 2",
		expected: "",
		init:     initFunc(`bot {app_telegram_token : "TEST_TOKEN}"`),
		positive: false,
		err:      ErrParseFile,
	}, {
		name:     "negative 3",
		expected: "",
		init:     initFunc(`{app_telegram_token = "TEST_TOKEN"}`),
		positive: false,
		err:      ErrParseFile,
	}, {
		name:     "negative 3",
		expected: "",
		init:     initFunc(`app_telegram_token = "TEST_TOKEN"`),
		positive: false,
		err:      ErrParseFile,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testFs := afero.NewMemMapFs()
			tc.init(testFs)
			cfg, err := NewBotConfig(testFs)
			if err != nil && tc.positive {
				require.Error(t, err, "err in positive test")
			} else if err != nil {
				require.EqualError(t, err, tc.err.Error())
			}
			if !tc.positive {
				assert.Error(t, errors.New("expected err"))
			} else {
				assert.Equal(t, tc.expected, cfg.TokenTGBot)
			}
		})
	}
}
