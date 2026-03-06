package config

import (
	"errors"

	"github.com/byrnedo/typesafe-config/parse"
	"github.com/spf13/afero"
)

const envFile = "app.env"

// BotConfig - struct with all that need 'Telegram Bot Service' to work
type BotConfig struct {
	TokenTGBot       string `config:"app_telegram_token"`
	ScrapperAddrGRPC string `config:"scrapper_addr_grpc"`
	BotAddrGRPC      string `config:"bot_addr_grpc"`
	BotAddrHTTP      string `config:"bot_addr_http"`
	TelegramDisabled bool   `config:"bot_disable_telegram"`
}

var (
	ErrorReadFile  = errors.New("error while read file")
	ErrorParseFile = errors.New("error while parse file")
)

// NewBotConfig - init and parse config file 'app.env' in root, with prefix 'bot'
func NewBotConfig(fs afero.Fs) (BotConfig, error) {
	var tree *parse.Tree
	var err error

	file, err := afero.ReadFile(fs, envFile)
	if err != nil {
		return BotConfig{}, ErrorReadFile
	}
	if tree, err = parse.ParseBytes(file); err != nil {
		return BotConfig{}, ErrorParseFile
	}
	config := &BotConfig{}
	parse.Populate(config, tree.GetConfig(), "bot")
	return *config, nil
}
