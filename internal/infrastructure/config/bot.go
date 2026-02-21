package config

import "github.com/byrnedo/typesafe-config/parse"

// BotConfig - struct with all that need 'Telegram Bot Service' to work
type BotConfig struct {
	TokenTGBot string `config:"app_telegram_token"`
}

// NewBotConfig - init and parse config file '.env' in root, with prefix 'bot'
func NewBotConfig() (BotConfig, error) {
	var tree *parse.Tree
	var err error
	if tree, err = parse.ParseFile(".env"); err != nil {
		return BotConfig{}, err
	}
	config := &BotConfig{}
	parse.Populate(config, tree.GetConfig(), "bot")
	return *config, err
}
