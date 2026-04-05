package stackoverflow

import "errors"

var (
	ErrInvalidQuestionURL = errors.New("invalid stackoverflow question url")
	ErrQuestionNotFound   = errors.New("stackoverflow question not found")
)
