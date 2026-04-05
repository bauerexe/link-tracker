package github

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Checker struct {
	GitHubClient     Client
	GitHubRepository Repository
	Log              *zap.Logger

	repoRe *regexp.Regexp
}

type repoCheckData struct {
	owner        string
	repo         string
	repoFullName string
	state        *State
	repository   *Repo
	issues       []*Issue
}

type issueSummary struct {
	newIssues              int
	newPRs                 int
	maxIssueNumber         int
	latestUpdateRepository time.Time
	issueItems             []*Issue
	prItems                []*Issue
}

const (
	matchParts = 3
	limitDesc  = 200
)

func NewChecker(githubClient Client, githubRepository Repository, log *zap.Logger) (*Checker, error) {
	if githubClient == nil {
		return nil, errors.New("GitHub client is nil")
	}
	if githubRepository == nil {
		return nil, errors.New("GitHub repository is nil")
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &Checker{
		GitHubClient:     githubClient,
		GitHubRepository: githubRepository,
		Log:              log,
		repoRe:           regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/?#]+)`),
	}, nil
}

func (g Checker) Match(url string) bool {
	return g.repoRe.Match([]byte(url))
}

func (g Checker) Check(ctx context.Context, url string, since time.Time,
) (desc string, updatedAt time.Time, updated bool, err error) {
	data, err := g.loadCheckData(ctx, url)
	if err != nil {
		return "", time.Time{}, false, err
	}

	summary := summarizeIssues(data.state, data.repository, data.issues, since)

	updated = g.isUpdated(data.state, data.repository, summary)
	newState := buildNewState(data.repository, summary.maxIssueNumber)

	if err = g.updateState(ctx, data.repoFullName, url, data.owner, newState); err != nil {
		return "", time.Time{}, false, err
	}

	if !updated {
		return "", summary.latestUpdateRepository, false, nil
	}

	desc = buildDescription(data.repoFullName, summary)
	return desc, summary.latestUpdateRepository, true, nil
}

func (g Checker) loadCheckData(ctx context.Context, url string) (*repoCheckData, error) {
	owner, repo, repoFullName, err := g.parseRepoURL(url)
	if err != nil {
		return nil, err
	}

	st, err := g.GitHubRepository.GetState(ctx, repoFullName)
	if err != nil {
		return nil, fmt.Errorf("get github state: %w", err)
	}

	g.logCurrentState(st, url, owner)

	repository, err := g.GitHubClient.GetRepo(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get github repo: %w", err)
	}

	issues, err := g.GitHubClient.ListIssuesAfter(ctx, owner, repo, st.LastProcessedIssueNumber)
	if err != nil {
		return nil, fmt.Errorf("list github issues: %w", err)
	}

	return &repoCheckData{
		owner:        owner,
		repo:         repo,
		repoFullName: repoFullName,
		state:        st,
		repository:   repository,
		issues:       issues,
	}, nil
}

func (g Checker) parseRepoURL(url string) (owner, repo, repoFullName string, err error) {
	matches := g.repoRe.FindStringSubmatch(url)

	if len(matches) < matchParts {
		return "", "", "", fmt.Errorf("invalid github repo url: %s", url)
	}

	owner = matches[1]
	repo = matches[2]
	repoFullName = owner + "/" + repo

	return owner, repo, repoFullName, nil
}

func (g Checker) logCurrentState(st *State, url, owner string) {
	lastUpdated := ""
	if st.LastRepoUpdated != nil {
		lastUpdated = st.LastRepoUpdated.Format(time.RFC3339)
	}

	g.Log.Info("Check github repo state",
		zap.String("LastRepoUpdated", lastUpdated),
		zap.String("repo url", url),
		zap.String("repo owner", owner),
	)
}

func summarizeIssues(st *State, repository *Repo, issues []*Issue, since time.Time) issueSummary {
	summary := issueSummary{
		maxIssueNumber:         st.LastProcessedIssueNumber,
		latestUpdateRepository: repository.UpdatedAt,
	}

	if st.LastRepoUpdated != nil && st.LastRepoUpdated.After(summary.latestUpdateRepository) {
		summary.latestUpdateRepository = *st.LastRepoUpdated
	}

	for _, issue := range issues {
		if shouldSkipIssue(issue, since, st.LastProcessedIssueNumber) {
			continue
		}

		updateMaxIssueNumber(&summary, issue)
		updateLatestRepositoryTime(&summary, issue)

		if isPullRequest(issue) {
			summary.newPRs++
			summary.prItems = append(summary.prItems, issue)
			continue
		}

		summary.newIssues++
		summary.issueItems = append(summary.issueItems, issue)
	}

	return summary
}

func shouldSkipIssue(issue *Issue, since time.Time, lastIssueNumber int) bool {
	if issue == nil {
		return true
	}
	if issue.CreatedAt.Before(since) {
		return true
	}
	if issue.Number <= lastIssueNumber {
		return true
	}

	return false
}

func updateMaxIssueNumber(summary *issueSummary, issue *Issue) {
	if issue.Number > summary.maxIssueNumber {
		summary.maxIssueNumber = issue.Number
	}
}

func updateLatestRepositoryTime(summary *issueSummary, issue *Issue) {
	if issue.CreatedAt.After(summary.latestUpdateRepository) {
		summary.latestUpdateRepository = issue.CreatedAt
	}
}

func isPullRequest(issue *Issue) bool {
	return issue.PullRequest.URL != ""
}

func (g Checker) isUpdated(st *State, repository *Repo, summary issueSummary) bool {
	repoChanged := st.LastRepoUpdated == nil || repository.UpdatedAt.After(*st.LastRepoUpdated)
	issuesChanged := summary.newIssues > 0 || summary.newPRs > 0

	return repoChanged || issuesChanged
}

func buildNewState(repository *Repo, maxIssueNumber int) State {
	return State{
		LastRepoUpdated:          &repository.UpdatedAt,
		LastProcessedIssueNumber: maxIssueNumber,
	}
}

func (g Checker) updateState(ctx context.Context, repoFullName string, url string, owner string, newState State,
) error {
	lastUpdated := ""
	if newState.LastRepoUpdated != nil {
		lastUpdated = newState.LastRepoUpdated.Format(time.RFC3339)
	}

	g.Log.Info("New state github repo state",
		zap.String("LastRepoUpdated", lastUpdated),
		zap.String("repo url", url),
		zap.String("repo owner", owner),
	)

	if err := g.GitHubRepository.UpdateState(ctx, repoFullName, newState); err != nil {
		return fmt.Errorf("update github state: %w", err)
	}

	return nil
}

func buildDescription(repoFullName string, summary issueSummary) string {
	switch {
	case summary.newIssues > 0 && summary.newPRs > 0:
		return buildIssuesAndPRsDescription(repoFullName, summary)
	case summary.newIssues > 0:
		return buildIssuesDescription(repoFullName, summary)
	case summary.newPRs > 0:
		return buildPRsDescription(repoFullName, summary)
	default:
		return fmt.Sprintf("Репозиторий %s был обновлен", repoFullName)
	}
}

func buildIssuesAndPRsDescription(repoFullName string, summary issueSummary) string {
	return fmt.Sprintf(
		"Обновления в %s:\n\nНовые issues (%d):\n%s\n\nНовые pull requests (%d):\n%s",
		repoFullName,
		summary.newIssues,
		joinIssueLines(summary.issueItems, formatIssueLine),
		summary.newPRs,
		joinIssueLines(summary.prItems, formatPRLine),
	)
}

func buildIssuesDescription(repoFullName string, summary issueSummary) string {
	return fmt.Sprintf(
		"Обновления в %s:\n\nНовые issues (%d):\n%s",
		repoFullName,
		summary.newIssues,
		joinIssueLines(summary.issueItems, formatIssueLine),
	)
}

func buildPRsDescription(repoFullName string, summary issueSummary) string {
	return fmt.Sprintf(
		"Обновления в %s:\n\nНовые pull requests (%d):\n%s",
		repoFullName,
		summary.newPRs,
		joinIssueLines(summary.prItems, formatPRLine),
	)
}

func joinIssueLines(items []*Issue, formatter func(*Issue) string) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, formatter(item))
	}

	return strings.Join(parts, "\n\n")
}

func previewText(s string, limit int) string {
	if s == "" {
		return "-"
	}

	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}

	return string(runes[:limit]) + "..."
}

func formatIssueLine(issue *Issue) string {
	return fmt.Sprintf(
		"#%d %s\nАвтор: %s\nСоздано: %s\nОписание: %s\nСсылка: %s",
		issue.Number,
		issue.Title,
		issue.User.Login,
		issue.CreatedAt.UTC().Format(time.RFC3339),
		previewText(issue.Body, limitDesc),
		issue.HTMLURL,
	)
}

func formatPRLine(issue *Issue) string {
	url := issue.HTMLURL
	if issue.PullRequest.HTMLURL != "" {
		url = issue.PullRequest.HTMLURL
	}

	return fmt.Sprintf(
		"#%d %s\nАвтор: %s\nСоздано: %s\nОписание: %s\nСсылка: %s",
		issue.Number,
		issue.Title,
		issue.User.Login,
		issue.CreatedAt.UTC().Format(time.RFC3339),
		previewText(issue.Body, limitDesc),
		url,
	)
}
