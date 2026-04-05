package github

import "errors"

var (
	ErrStateAlreadyExists = errors.New("already Exists")
	ErrStateNotFound      = errors.New("not Found")
)
