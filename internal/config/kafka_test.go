package config

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKafkaConfig(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		name     string
		expected KafkaConfig
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
			expected: KafkaConfig{
				KafkaTopic:         "updates",
				KafkaConsumerGroup: "scrapper-group",
				KafkaBrokers:       []string{"localhost:9092", "localhost:9093"},
				KafkaEnabled:       true,
			},
			init: initFunc(`
kafka {
  kafka_topic = "updates"
  kafka_consumer_group = "scrapper-group"
  kafka_brokers = ["localhost:9092", "localhost:9093"]
  kafka_enabled = true
}
`),
		},
		{
			name: "positive partial fields",
			expected: KafkaConfig{
				KafkaTopic:   "links",
				KafkaEnabled: false,
			},
			init: initFunc(`
kafka {
  kafka_topic = "links"
  kafka_enabled = false
}
`),
		},
		{
			name: "no kafka prefix -> empty cfg, no error",
			init: initFunc(`
kafka_topic = "updates"
kafka_enabled = true
`),
			expected: KafkaConfig{},
		},
		{
			name: "negative parse error",
			init: initFunc(`
kafka {
  kafka_topic = "updates
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

			cfg, err := NewKafkaConfig(testFs)

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
