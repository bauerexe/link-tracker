package domain

import (
	"time"
)

// Chat - chat from tg
type Chat struct {
	ID int64
}

// Link - link to url with tags and filters
type Link struct {
	ID      int64
	URL     string
	Tags    []string
	Filters []string
}

// URLState - for check updates time
type URLState struct {
	LastCheckedAt time.Time
	LastUpdatedAt time.Time
}
