package stackoverflow

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Checker struct {
	Client Client
	Log    *zap.Logger

	questionRe *regexp.Regexp
}

const (
	stackOverflowQuestionMatchSize = 2
	previewLimit                   = 200
)

func NewChecker(client Client, log *zap.Logger) (*Checker, error) {
	if client == nil {
		return nil, errors.New("stackoverflow client is nil")
	}
	if log == nil {
		log = zap.NewNop()
	}

	return &Checker{
		Client:     client,
		Log:        log,
		questionRe: regexp.MustCompile(`^https?://stackoverflow\.com/questions/(\d+)(/|$)`),
	}, nil
}

func (c *Checker) Match(url string) bool {
	return c.questionRe.MatchString(url)
}

func (c *Checker) Check(ctx context.Context, url string, since time.Time) (desc string, updatedAt time.Time, updated bool, err error) {
	questionID, err := c.parseQuestionID(url)
	if err != nil {
		return "", time.Time{}, false, err
	}

	question, err := c.Client.GetQuestion(ctx, questionID)
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("get stackoverflow question: %w", err)
	}

	answers, err := c.Client.ListAnswers(ctx, questionID)
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("list stackoverflow answers: %w", err)
	}

	comments, err := c.Client.ListComments(ctx, questionID)
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("list stackoverflow comments: %w", err)
	}

	questionUpdatedAt := time.Unix(question.LastActivityDate, 0).UTC()
	events := collectNewEvents(question, answers, comments, since.UTC())

	if len(events) == 0 {
		return "", questionUpdatedAt, false, nil
	}

	slices.SortFunc(events, func(a, b activityItem) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})

	desc = buildDescription(question, events)
	updatedAt = maxUpdatedAt(questionUpdatedAt, events)

	return desc, updatedAt, true, nil
}

func (c *Checker) parseQuestionID(url string) (string, error) {
	matches := c.questionRe.FindStringSubmatch(url)
	if len(matches) < stackOverflowQuestionMatchSize {
		return "", fmt.Errorf("%w: %s", ErrInvalidQuestionURL, url)
	}

	return matches[1], nil
}

type activityItem struct {
	Author    string
	CreatedAt time.Time
	Body      string
	Link      string
}

func collectNewEvents(question *Question, answers, comments []AnswerOrComment, since time.Time) []activityItem {
	result := make([]activityItem, 0)

	for _, answer := range answers {
		createdAt := time.Unix(answer.CreationDate, 0).UTC()
		if !since.IsZero() && !createdAt.After(since) {
			continue
		}

		result = append(result, activityItem{
			Author:    answer.Owner.DisplayName,
			CreatedAt: createdAt,
			Body:      answer.Body,
			Link:      fallbackLink(answer.Link, question.Link),
		})
	}

	for _, comment := range comments {
		createdAt := time.Unix(comment.CreationDate, 0).UTC()
		if !since.IsZero() && !createdAt.After(since) {
			continue
		}

		result = append(result, activityItem{
			Author:    comment.Owner.DisplayName,
			CreatedAt: createdAt,
			Body:      comment.Body,
			Link:      fallbackLink(comment.Link, question.Link),
		})
	}

	return result
}

func maxUpdatedAt(questionUpdatedAt time.Time, items []activityItem) time.Time {
	maxTime := questionUpdatedAt.UTC()

	for _, item := range items {
		if item.CreatedAt.After(maxTime) {
			maxTime = item.CreatedAt.UTC()
		}
	}

	return maxTime
}

func buildDescription(question *Question, items []activityItem) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, formatActivityItem(item))
	}

	return fmt.Sprintf(
		"Обновления в StackOverflow:\n\nВопрос: %s\n\nНовые ответы/комментарии (%d):\n%s",
		question.Title,
		len(items),
		strings.Join(lines, "\n\n"),
	)
}

func formatActivityItem(item activityItem) string {
	return fmt.Sprintf(
		"Автор: %s\nСоздано: %s\nОписание: %s\nСсылка: %s",
		fallbackString(item.Author, "-"),
		item.CreatedAt.UTC().Format(time.RFC3339),
		previewText(stripHTML(item.Body), previewLimit),
		item.Link,
	)
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

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func fallbackLink(link, questionLink string) string {
	if strings.TrimSpace(link) == "" {
		return questionLink
	}
	return link
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	return strings.Join(
		strings.Fields(
			html.UnescapeString(
				htmlTagRe.ReplaceAllString(s, " "),
			),
		), " ")
}
