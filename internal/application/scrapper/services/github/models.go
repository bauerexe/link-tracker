package github

import (
	"time"
)

type Repo struct {
	FullName        string    `json:"full_name"`
	HTMLURL         string    `json:"html_url"`
	UpdatedAt       time.Time `json:"updated_at"`
	HasIssues       bool      `json:"has_issues"`
	OpenIssuesCount int       `json:"open_issues_count"`
	IssuesURL       string    `json:"issues_url"`
}

type User struct {
	Login   string `json:"login"`
	HTMLURL string `json:"html_url"`
}
type Issue struct {
	Number      int         `json:"number"`
	Title       string      `json:"title"`
	Body        string      `json:"body"`
	HTMLURL     string      `json:"html_url"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	User        User        `json:"user"`
	PullRequest PullRequest `json:"pull_request"`
}

type PullRequest struct {
	URL     string `json:"url"`
	HTMLURL string `json:"html_url"`
}

type State struct {
	LastRepoUpdated          *time.Time `db:"last_repo_updated_at"`
	LastProcessedIssueNumber int        `db:"last_processed_issue_number"`
}
