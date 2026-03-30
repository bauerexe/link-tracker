package domain

// Command - the command from bot
type Command string

func (c Command) String() string {
	return string(c)
}

// Handler - handler of Command
type Handler interface {
	Handle(chatID int64, args string) (string, error)
}

// Message - the struct with info about message
type Message struct {
	ChatID    int64
	Text      string
	Command   Command
	Arguments string
	MessageID int
}
