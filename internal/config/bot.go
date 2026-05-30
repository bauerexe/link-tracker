package config

import (
	"errors"
	"os"

	"github.com/byrnedo/typesafe-config/parse"
	"github.com/spf13/afero"
)

var EnvFile = "app.env"

func configPath() string {
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		return p
	}
	return EnvFile
}

// BotConfig - struct with all that need 'Telegram Bot Service' to work
type BotConfig struct {
	TokenTGBot       string `config:"app_telegram_token"`
	ScrapperAddrGRPC string `config:"scrapper_addr_grpc"`
	BotAddrGRPC      string `config:"bot_addr_grpc"`
	BotAddrHTTP      string `config:"bot_addr_http"`
	BotMetricsAddr   string `config:"bot_metrics_addr"`
	TelegramDisabled bool   `config:"bot_disable_telegram"`
}

var (
	ErrReadFile  = errors.New("error while read file")
	ErrParseFile = errors.New("error while parse file")
)

// NewBotConfig - init and parse config file 'app.env' in root, with prefix 'bot'
func NewBotConfig(fs afero.Fs) (BotConfig, error) {
	var tree *parse.Tree
	var err error

	file, err := afero.ReadFile(fs, configPath())
	if err != nil {
		return BotConfig{}, ErrReadFile
	}
	if tree, err = parse.ParseBytes(file); err != nil {
		return BotConfig{}, ErrParseFile
	}
	config := &BotConfig{}
	parse.Populate(config, tree.GetConfig(), "bot")
	return *config, nil
}
