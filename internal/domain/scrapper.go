package domain

type Chat struct {
	ID int64
}

type Link struct {
	ID      int64
	URL     string
	Tags    []string
	Filters []string
}
