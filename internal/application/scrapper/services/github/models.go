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

type Issue struct {
	CreatedAt   time.Time   `json:"created_at"`
	PullRequest PullRequest `json:"pull_request"`
}

type PullRequest struct {
	URL string `json:"url"`
}

type State struct {
	LastRepoUpdated          *time.Time `db:"last_repo_updated_at"`
	LastProcessesIssueNumber int        `db:"last_processes_issue_number"`
}
