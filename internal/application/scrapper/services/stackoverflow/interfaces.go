package stackoverflow

import "context"

type Client interface {
	GetQuestion(ctx context.Context, questionID string) (*Question, error)
	ListAnswers(ctx context.Context, questionID string) ([]AnswerOrComment, error)
	ListComments(ctx context.Context, questionID string) ([]AnswerOrComment, error)
}
