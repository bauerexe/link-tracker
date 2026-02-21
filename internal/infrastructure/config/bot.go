package config

import (
	"errors"

	"github.com/byrnedo/typesafe-config/parse"
	"github.com/spf13/afero"
)

// BotConfig - struct with all that need 'Telegram Bot Service' to work
type BotConfig struct {
	TokenTGBot string `config:"app_telegram_token"`
}

var (
	ErrorReadFile  = errors.New("error while read file")
	ErrorParseFile = errors.New("error while read file")
)

// NewBotConfig - init and parse config file '.env' in root, with prefix 'bot'
func NewBotConfig(fs afero.Fs) (BotConfig, error) {
	var tree *parse.Tree
	var err error
	file, err := afero.ReadFile(fs, ".env")
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
